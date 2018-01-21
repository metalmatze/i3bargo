package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os/exec"
	"regexp"
	"strconv"
	"time"
)

// Block is a container for the information that being displayed.
type Block struct {
	FullText            string `json:"full_text"`
	ShortText           string `json:"short_text,omitempty"`
	Color               string `json:"color,omitempty"`
	Background          string `json:"background,omitempty"`
	Border              string `json:"border,omitempty"`
	MinWidth            int    `json:"min_width,omitempty"`
	Align               string `json:"align,omitempty"`
	Urgent              bool   `json:"urgent,omitempty"`
	Name                string `json:"name,omitempty"`
	Instance            string `json:"instance,omitempty"`
	Separator           bool   `json:"separator,omitempty"`
	SeparatorBlockWidth int    `json:"separator_block_width,omitempty"`
}

// Update is an event send by funcs to update the state.
type Update struct {
	Place   int
	Content []byte
}

func main() {
	updates := make(chan Update)

	go datetime(updates)
	go uptime(updates)
	go temperature(updates)
	go volume(updates)

	state := make([][]byte, 4)

	fmt.Println(`{ "version": 1 }`)
	fmt.Println("[")
	for update := range updates {
		state[update.Place] = update.Content

		fmt.Println("[")
		for i, s := range state {
			if len(s) == 0 {
				s = []byte(`{"full_text":""}`)
			}

			comma := ""
			if i < len(state)-1 {
				comma = ","
			}

			fmt.Printf("\t%s%s\n", s, comma)
		}
		fmt.Println("],")
	}
}

func datetime(updates chan<- Update) {
	for {
		b := Block{
			FullText:            time.Now().Format("2006-01-02 15:04:05"),
			Separator:           true,
			SeparatorBlockWidth: 20,
		}

		out, err := json.Marshal(b)
		if err != nil {
			// TODO: figure out error handling
			return
		}

		updates <- Update{Place: 3, Content: out}

		time.Sleep(time.Second)
	}
}

func uptime(updates chan<- Update) {
	for {
		content, err := ioutil.ReadFile("/proc/uptime")
		if err != nil {
			// TODO: figure out error handling
			return
		}
		content = bytes.TrimSpace(content)
		contents := bytes.Split(content, []byte(" "))

		uptimeFloat, err := strconv.ParseFloat(string(contents[0]), 64)
		if err != nil {
			// TODO: figure out error handling
			return
		}

		uptime := time.Duration(uptimeFloat) * time.Second

		b := Block{
			FullText:            uptime.String(),
			Separator:           true,
			SeparatorBlockWidth: 20,
		}

		out, err := json.Marshal(b)
		if err != nil {
			// TODO: figure out error handling
			return
		}

		updates <- Update{Place: 2, Content: out}

		time.Sleep(10 * time.Second)
	}
}

func temperature(updates chan<- Update) {
	for {
		content, err := ioutil.ReadFile("/sys/class/hwmon/hwmon1/temp1_input")
		if err != nil {
			// TODO: figure out error handling
			return
		}
		content = bytes.TrimSpace(content)

		celsius, err := strconv.ParseInt(string(content), 10, 64)
		if err != nil {
			// TODO: figure out error handling
			return
		}

		b := Block{
			FullText:            fmt.Sprintf("%d°C", celsius/1000),
			Separator:           true,
			SeparatorBlockWidth: 20,
		}

		out, err := json.Marshal(b)
		if err != nil {
			// TODO: figure out error handling
			return
		}

		updates <- Update{Place: 1, Content: out}

		time.Sleep(5 * time.Second)
	}
}

var volumeRegex = regexp.MustCompile(`\[(\d{1,3})\%\]\s\[(on|off)\]`)

func volume(updates chan<- Update) {
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
		defer cancel() // TODO: Fix possible leak

		cmd := exec.CommandContext(ctx, "amixer", "-D", "default", "get", "Master")
		output, err := cmd.Output()
		if err != nil {
			// TODO: figure out error handling
			return
		}

		var volText, muteText string

		scanner := bufio.NewScanner(bytes.NewBuffer(output))
		for scanner.Scan() {
			line := scanner.Text()

			if volumeRegex.MatchString(line) {
				matches := volumeRegex.FindStringSubmatch(line)
				volText, muteText = matches[1], matches[2]
				break
			}
		}

		vol, err := strconv.ParseInt(volText, 10, 64)
		if err != nil {
			// TODO: figure out error handling
			return
		}

		muted := false
		if muteText == "off" {
			muted = true
		}

		fulltext := fmt.Sprintf("%d%%", vol)
		if muted {
			fulltext = "off"
		}

		b := Block{
			FullText:            fulltext,
			Separator:           true,
			SeparatorBlockWidth: 20,
		}

		out, err := json.Marshal(b)
		if err != nil {
			// TODO: figure out error handling
			return
		}

		updates <- Update{Place: 0, Content: out}

		time.Sleep(time.Second)
	}
}
