package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"time"

	batt "github.com/distatus/battery"
	"github.com/metalmatze/i3bargo/fontawesome"
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
	Place   uint
	Content json.RawMessage
	Error   error
}

type updater func(place uint, updates chan<- Update)

func main() {
	logs, err := ioutil.TempFile("", "i3bargo")
	if err != nil {
		fmt.Println("failed to open logs file:", err)
		os.Exit(1)
	}
	defer logs.Close()

	updates := make(chan Update)

	updaters := []updater{
		memoryUpdater,
		volumeUpdater,
		temperatureUpdater,
		batteryUpdater,
		uptimeUpdater,
		datetimeUpdater,
	}

	for i, updater := range updaters {
		go updater(uint(i), updates)
	}

	state := make([]json.RawMessage, len(updaters))

	fmt.Println(`{ "version": 1 }`)
	fmt.Println("[")
	for update := range updates {
		state[update.Place] = update.Content

		if update.Error != nil {
			logs.WriteString(fmt.Sprintf("error in updater: %v\n", update.Error))
			logs.Sync()
			state[update.Place] = json.RawMessage(`{"full_text":" error","separator":true,"separator_block_width":20}`)
		}

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

func batteryUpdater(place uint, updates chan<- Update) {
	for {
		out, err := battery()

		updates <- Update{
			Place:   place,
			Content: out,
			Error:   err,
		}

		time.Sleep(time.Second)
	}
}

func battery() (json.RawMessage, error) {
	b, err := batt.Get(0)
	if err != nil {
		return nil, err // TODO: Use errors.Wrap
	}

	w := &bytes.Buffer{}

	w.WriteString(fmt.Sprintf("%s ", fontawesome.BatteryFull))

	fmt.Fprintf(w, "%.0f%%", (b.Current/b.Full)*100)

	if b.Current != b.Full {
		d, err := time.ParseDuration(fmt.Sprintf("%fh", b.Current/b.ChargeRate))
		if err != nil {
			return nil, err // TODO: Use errors.Wrap
		}

		w.WriteString(" - ")

		if d.Hours() > 1 {
			fmt.Fprintf(w, "%dh", int(d.Hours()))
		} else {
			fmt.Fprintf(w, "%dm", int(d.Minutes()))
		}
	}

	block := Block{
		FullText:            w.String(),
		Separator:           true,
		SeparatorBlockWidth: 20,
	}

	return json.Marshal(block)
}

func datetimeUpdater(place uint, updates chan<- Update) {
	for {
		out, err := datetime()

		updates <- Update{
			Place:   place,
			Content: out,
			Error:   err,
		}

		time.Sleep(time.Second)
	}
}

func datetime() (json.RawMessage, error) {
	b := Block{
		FullText:            time.Now().Format("2006-01-02 15:04:05"),
		Separator:           true,
		SeparatorBlockWidth: 20,
	}

	return json.Marshal(b)
}

func uptimeUpdater(place uint, updates chan<- Update) {
	for {
		out, err := uptime()

		updates <- Update{
			Place:   place,
			Content: out,
			Error:   err,
		}

		time.Sleep(10 * time.Second)
	}
}

func uptime() (json.RawMessage, error) {
	content, err := ioutil.ReadFile("/proc/uptime")
	if err != nil {
		return nil, err // TODO: Use errors.Wrap
	}
	content = bytes.TrimSpace(content)
	contents := bytes.Split(content, []byte(" "))

	uptimeFloat, err := strconv.ParseFloat(string(contents[0]), 64)
	if err != nil {
		return nil, err // TODO: Use errors.Wrap
	}

	uptime := time.Duration(uptimeFloat) * time.Second

	b := Block{
		FullText:            fmt.Sprintf("%s %s", fontawesome.ArrowCircleUp, uptime.String()),
		Separator:           true,
		SeparatorBlockWidth: 20,
	}

	return json.Marshal(b)
}

func temperatureUpdater(place uint, updates chan<- Update) {
	for {
		out, err := temperature()

		updates <- Update{
			Place:   place,
			Content: out,
			Error:   err,
		}

		time.Sleep(5 * time.Second)
	}
}

func temperature() (json.RawMessage, error) {
	content, err := ioutil.ReadFile("/sys/class/hwmon/hwmon1/temp1_input")
	if err != nil {
		return nil, err
	}
	content = bytes.TrimSpace(content)

	celsius, err := strconv.ParseInt(string(content), 10, 64)
	if err != nil {
		return nil, err
	}

	b := Block{
		FullText:            fmt.Sprintf("%s %d°C", fontawesome.ThermometerFull, celsius/1000),
		Separator:           true,
		SeparatorBlockWidth: 20,
	}

	return json.Marshal(b)
}

func volumeUpdater(place uint, updates chan<- Update) {
	for {
		out, err := volume()

		updates <- Update{
			Place:   place,
			Content: out,
			Error:   err,
		}

		time.Sleep(time.Second)
	}
}

var volumeRegex = regexp.MustCompile(`\[(\d{1,3})\%\]\s\[(on|off)\]`)

func volume() (json.RawMessage, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(ctx, "amixer", "-D", "default", "get", "Master")
	output, err := cmd.Output()
	if err != nil {
		return nil, err // TODO: Use errors.Wrap
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
		return nil, err // TODO: Use errors.Wrap

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
		FullText:            fmt.Sprintf("%s %s", fontawesome.VolumeUp, fulltext),
		Separator:           true,
		SeparatorBlockWidth: 20,
	}

	return json.Marshal(b)
}

func memoryUpdater(place uint, updates chan<- Update) {
	for {
		out, err := memory()

		updates <- Update{
			Place:   place,
			Content: out,
			Error:   err,
		}

		time.Sleep(time.Second)
	}
}

func memory() (json.RawMessage, error) {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return nil, err // TODO: Use errors.Wrap
	}
	defer file.Close()

	var total, free, available float64
	_, err = fmt.Fscanf(file,
		"MemTotal: %f kB\nMemFree: %f kB\nMemAvailable: %f",
		&total,
		&free,
		&available,
	)
	if err != nil {
		return nil, err // TODO: Use errors.Wrap
	}

	b := Block{
		FullText:            fmt.Sprintf("%s %.2fG", fontawesome.Microchip, available/(1024*1024)),
		Separator:           true,
		SeparatorBlockWidth: 20,
	}

	return json.Marshal(b)
}
