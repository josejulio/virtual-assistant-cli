package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
)

type Output interface {
	Messages() []OutputMessage
	SessionId() string
}

type OutputImpl struct {
	messages  []OutputMessage
	sessionId string
}

func (o *OutputImpl) Messages() []OutputMessage {
	return o.messages
}

func (o *OutputImpl) SessionId() string {
	return o.sessionId
}

type options struct {
	Text  string
	Value string
}

type OutputMessage struct {
	Text    *string
	Options []options
	Pause   int
	Command *string
}

type Input struct {
	Message   string
	SessionId *string
}

type response struct {
	Session  string           `json:"session_id"`
	Response []map[string]any `json:"response"`
}

func MakeSendMessageFn(restApi *url.URL) func(Input) (Output, error) {
	return func(input Input) (Output, error) {
		baseURL := restApi

		values := map[string]any{"input": map[string]string{"text": input.Message}, "session_id": input.SessionId}
		jsonValues, err := json.Marshal(values)
		if err != nil {
			return nil, err
		}

		// resp, err := http.Post(baseURL.String(), "application/json", bytes.NewBuffer(jsonValues))
		request, err := http.NewRequest("POST", baseURL.String(), bytes.NewBuffer(jsonValues))
		if err != nil {
			return nil, err
		}

		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("x-rh-identity", "eyJpZGVudGl0eSI6IHsiYWNjb3VudF9udW1iZXIiOiJhY2NvdW50MTIzIiwib3JnX2lkIjoib3JnMTIzIiwidHlwZSI6IlVzZXIiLCJ1c2VyIjp7ImlzX29yZ19hZG1pbiI6dHJ1ZSwgInVzZXJfaWQiOiIxMjM0NTY3ODkwIiwidXNlcm5hbWUiOiJhc3RybyJ9LCJpbnRlcm5hbCI6eyJvcmdfaWQiOiJvcmcxMjMifX19")

		resp, err := http.DefaultClient.Do(request)
		if err != nil {
			return nil, err
		}

		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("received non-200 status code: %d", resp.StatusCode)
		}

		var assistantResponse response
		if err = json.Unmarshal(body, &assistantResponse); err != nil {
			return nil, err
		}

		var outputMessages []OutputMessage
		for _, message := range assistantResponse.Response {
			if message["type"] == "TEXT" {
				text := message["text"].(string)
				outputMessages = append(outputMessages, OutputMessage{
					Text: &text,
				})
			} else if message["type"] == "OPTIONS" {
				var text *string
				if message["text"] != nil {
					localText := message["text"].(string)
					text = &localText
				}
				var ops []options
				for _, option := range message["options"].([]interface{}) {
					mapOptions := option.(map[string]interface{})
					ops = append(ops, options{
						Text:  mapOptions["text"].(string),
						Value: mapOptions["value"].(string),
					})
					// print(option)
				}
				outputMessages = append(outputMessages, OutputMessage{
					Text:    text,
					Options: ops,
				})
			} else if message["type"] == "PAUSE" {
				outputMessages = append(outputMessages, OutputMessage{
					Pause: message["time"].(int),
				})
			} else if message["type"] == "COMMAND" {
				command := message["command"].(string)
				outputMessages = append(outputMessages, OutputMessage{
					Command: &command,
				})
			}
		}

		return &OutputImpl{
			messages:  outputMessages,
			sessionId: assistantResponse.Session,
		}, nil
	}
}
