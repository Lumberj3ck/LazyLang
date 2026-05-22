package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"
)

type ExerciseTopic struct {
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	Desc       string   `json:"desc"`
	TargetRole string   `json:"target_role"`
	Guide      []string `json:"guide"`
}

func RandomTopicByLevel(level string) (ExerciseTopic, error) {
	data, err := os.ReadFile("topics.json")
	if err != nil {
		return ExerciseTopic{}, err
	}

	var exercisesByLevel map[string][]ExerciseTopic
	if err := json.Unmarshal(data, &exercisesByLevel); err != nil {
		return ExerciseTopic{}, fmt.Errorf("failed to parse ind.js JSON: %w", err)
	}

	normalizedLevel := strings.ToLower(strings.TrimSpace(level))
	topics, ok := exercisesByLevel[normalizedLevel]
	if !ok {
		return ExerciseTopic{}, fmt.Errorf("level %s was not found in ind.js", level)
	}
	if len(topics) == 0 {
		return ExerciseTopic{}, fmt.Errorf("no topics found for level %s", level)
	}

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return topics[r.Intn(len(topics))], nil
}

func FormatTopicForChat(topic ExerciseTopic) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Topic: %s\n", topic.Title))
	b.WriteString(fmt.Sprintf("Scenario: %s\n", topic.Desc))
	b.WriteString(fmt.Sprintf("Role: %s\n", topic.TargetRole))
	b.WriteString("Guide:\n")
	for i, step := range topic.Guide {
		b.WriteString(fmt.Sprintf("%d. %s\n", i+1, step))
	}
	return strings.TrimSpace(b.String())
}
