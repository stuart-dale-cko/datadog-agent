// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-2020 Datadog, Inc.

package processor

import (
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/DataDog/datadog-agent/pkg/trace/obfuscate"
	"github.com/DataDog/datadog-agent/pkg/util/log"

	"github.com/DataDog/datadog-agent/pkg/logs/config"
	"github.com/DataDog/datadog-agent/pkg/logs/message"
	"github.com/DataDog/datadog-agent/pkg/logs/metrics"
)

// A Processor updates messages from an inputChan and pushes
// in an outputChan.
type Processor struct {
	inputChan       chan *message.Message
	outputChan      chan *message.Message
	processingRules []*config.ProcessingRule
	encoder         Encoder
	done            chan struct{}
}

// New returns an initialized Processor.
func New(inputChan, outputChan chan *message.Message, processingRules []*config.ProcessingRule, encoder Encoder) *Processor {
	return &Processor{
		inputChan:       inputChan,
		outputChan:      outputChan,
		processingRules: processingRules,
		encoder:         encoder,
		done:            make(chan struct{}),
	}
}

// Start starts the Processor.
func (p *Processor) Start() {
	go p.run()
}

// Stop stops the Processor,
// this call blocks until inputChan is flushed
func (p *Processor) Stop() {
	close(p.inputChan)
	<-p.done
}

// run starts the processing of the inputChan
func (p *Processor) run() {
	defer func() {
		p.done <- struct{}{}
	}()
	for msg := range p.inputChan {
		metrics.LogsDecoded.Add(1)
		metrics.TlmLogsDecoded.Inc()
		if shouldProcess, redactedMsg := p.applyRedactingRules(msg); shouldProcess {
			metrics.LogsProcessed.Add(1)
			metrics.TlmLogsProcessed.Inc()

			// Encode the message to its final format
			content, err := p.encoder.Encode(msg, redactedMsg)
			if err != nil {
				log.Error("unable to encode msg ", err)
				continue
			}
			msg.Content = content
			p.outputChan <- msg
		}
	}
}

var obfuscator = obfuscate.NewObfuscator(nil)

type submatchGroup struct {
	name  string
	start int
	end   int
}

func submatchGroups(r *regexp.Regexp, content []byte) []submatchGroup {
	// first group is always empty
	groups := r.SubexpNames()[1:]
	submatches := r.FindSubmatchIndex(content)
	result := make([]submatchGroup, len(groups), len(groups))
	for i, g := range groups {
		si := 2 + i*2
		result[i] = submatchGroup{name: g, start: submatches[si], end: submatches[si+1]}
	}
	return result
}

type pgAutoExplain struct {
	// postgres original query text if logged from auto_explain
	QueryText string `json:"Query Text"`
}

func applyObfuscateSqlRule(r *regexp.Regexp, content []byte) (result []byte, err error) {
	groups := submatchGroups(r, content)
	rawQuery := ""
	for _, g := range groups {
		switch g.name {
		case "query", "query_raw":
			rawQuery = string(content[g.start:g.end])
		case "auto_explain_json":
			var plan pgAutoExplain
			if err := json.Unmarshal(content[g.start:g.end], &plan); err != nil {
				return nil, fmt.Errorf("failed to unmarshal json execution plan='%s' error='%s'", content[g.start:g.end], err)
			}
			if plan.QueryText != "" {
				rawQuery = plan.QueryText
			}
		}
	}
	if rawQuery == "" {
		return
	}
	obfQuery, err := obfuscator.ObfuscateSQLString(rawQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to obfuscate sql query='%s' error='%s'", rawQuery, err)
	}
	ci := 0
	for _, g := range groups {
		result = append(result, content[ci:g.start]...)
		ci = g.start
		if g.name == "query" {
			result = append(result, []byte(obfQuery.Query)...)
			ci = g.end
		} else if g.name == "query_raw" {
			result = append(result, content[g.start:g.end]...)
			ci = g.end
		} else if g.name == "sig_insert" {
			result = append(result, []byte(fmt.Sprintf(" %s ", obfuscate.HashObfuscatedSql(rawQuery)))...)
		}
	}
	if ci < len(content) {
		result = append(result, content[ci:]...)
	}
	return result, nil
}

// applyRedactingRules returns given a message if we should process it or not,
// and a copy of the message with some fields redacted, depending on config
func (p *Processor) applyRedactingRules(msg *message.Message) (bool, []byte) {
	content := msg.Content
	rules := append(p.processingRules, msg.Origin.LogSource.Config.ProcessingRules...)
	for _, rule := range rules {
		switch rule.Type {
		case config.ExcludeAtMatch:
			if rule.Regex.Match(content) {
				return false, nil
			}
		case config.IncludeAtMatch:
			if !rule.Regex.Match(content) {
				return false, nil
			}
		case config.MaskSequences:
			content = rule.Regex.ReplaceAll(content, rule.Placeholder)
		case config.ObfuscateSql:
			c, err := applyObfuscateSqlRule(rule.Regex, content)
			if err != nil {
				log.Errorf("failed to obfuscate sql query='%s', error: %s", content, err)
			} else if len(c) > 0 {
				content = c
			}
		}
	}
	return true, content
}
