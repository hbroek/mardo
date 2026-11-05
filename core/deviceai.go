package core

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"mardo/internal/ai"
)

// DeviceAI is what a host offers when the device has a language model of its own (Apple Intelligence on an iPhone).
// Such a model needs no key and no server, and the text never leaves the device.
type DeviceAI interface {
	// DeviceAIStatus is "" when the model can be used, else why it cannot (not supported, turned off, still downloading).
	DeviceAIStatus() string
	// DeviceAIChat answers a conversation, given as JSON: [{"role": "system", "content": "..."}, ...].
	DeviceAIChat(messagesJSON string) (string, error)
}

const (
	deviceAIPreset = "device"
	deviceAIURL    = "device://apple" // what the settings keep as the address of the device's own model
	// deviceAIRunes is how much text the device model is sent at once: its window is about 4000 tokens, which
	// the answer shares.
	deviceAIRunes = 8000
)

func isDeviceURL(baseURL string) bool { return strings.TrimSpace(baseURL) == deviceAIURL }

// devicePreset is the entry in the provider list for the device's own model.
func devicePreset() ai.Preset {
	return ai.Preset{ID: deviceAIPreset, Name: "Apple Intelligence (on this device)", BaseURL: deviceAIURL,
		Note: "Runs on this device: the page text never leaves it, and no key is needed. It is a small model that reads only the first few " +
			"pages of text, so long pages are cut short and answers are plainer than a large cloud model's. Choose another provider for those."}
}

// deviceAI is the host's own model and why it cannot be used, "" when it can.
func (a *Core) deviceAI() (DeviceAI, string, bool) {
	d, ok := a.host.(DeviceAI)
	if !ok {
		return nil, "", false
	}
	return d, d.DeviceAIStatus(), true
}

// deviceAvailable says whether there is a device model to use.
func (a *Core) deviceAvailable() bool {
	_, why, ok := a.deviceAI()
	return ok && why == ""
}

// deviceModel is the device's model as an AI provider.
func (a *Core) deviceModel() (ai.Native, error) {
	d, why, ok := a.deviceAI()
	if !ok {
		return ai.Native{}, errors.New("this device has no AI model of its own")
	}
	if why != "" {
		return ai.Native{}, errors.New(why)
	}
	return ai.Native{MaxRunes: deviceAIRunes, Run: func(ctx context.Context, messages []ai.Message) (string, error) {
		raw, err := json.Marshal(messages)
		if err != nil {
			return "", err
		}
		type result struct {
			text string
			err  error
		}
		done := make(chan result, 1)
		go func() {
			text, err := d.DeviceAIChat(string(raw))
			done <- result{text, err}
		}()
		select {
		case <-ctx.Done(): // the model finishes in the background and its answer is dropped
			return "", ctx.Err()
		case r := <-done:
			return r.text, r.err
		}
	}}, nil
}
