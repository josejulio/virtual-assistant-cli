package main

import (
	"fmt"
	"github.com/spf13/viper"
	"log"
	"net/url"
	"virtual-assistant-cli/internal/api"
	"virtual-assistant-cli/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	viper.AddConfigPath(".")
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")

	err := viper.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("fatal error config file: %w", err))
	}

	baseURL, err := url.Parse(viper.Get("server").(string) + "/api/virtual-assistant/v2/talk") // Replace with your URL
	if err != nil {
		panic(err)
	}
	
	p := tea.NewProgram(ui.CreateModel(api.MakeSendMessageFn(baseURL, api.Config{Debug: api.ConfigDebug{
		Enabled:          viper.GetBool("debug.enabled"),
		IncludeAssistant: viper.GetBool("debug.include_assistant"),
		IncludeResponse:  viper.GetBool("debug.include_response"),
	}})))
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}
