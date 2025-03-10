package ui

import (
	"fmt"
	"io"
	"strings"
	"virtual-assistant-cli/internal/api"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const focusedLoading = "loading"
const focusedInput = "input"
const focusedPicker = "picker"

type Model struct {
	messages  []string
	textInput textinput.Model
	picker    *list.Model
	loading   bool
	err       string
	spinner   spinner.Model

	sendMessage func(input api.Input) tea.Cmd
	session     *string

	userStyle  lipgloss.Style
	astroStyle lipgloss.Style
}

var (
	itemStyle         = lipgloss.NewStyle().PaddingLeft(4)
	selectedItemStyle = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("170"))
)

type listElement struct {
	text  string
	value string
}

func (i listElement) FilterValue() string { return "" }

type itemDelegate struct{}

func (d itemDelegate) Height() int                             { return 1 }
func (d itemDelegate) Spacing() int                            { return 0 }
func (d itemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(listElement)
	if !ok {
		return
	}

	str := fmt.Sprintf("%d. %s", index+1, i.text)

	fn := itemStyle.Render
	if index == m.Index() {
		fn = func(s ...string) string {
			return selectedItemStyle.Render("> " + strings.Join(s, " "))
		}
	}

	fmt.Fprint(w, fn(str))
}

func (m Model) focused() string {
	if m.loading {
		return focusedLoading
	} else if m.picker != nil {
		return focusedPicker
	}

	return focusedInput
}

func CreateModel(sendMessage func(api.Input) (api.Output, error)) Model {
	ti := textinput.New()
	ti.Placeholder = "Send a message..."
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 44

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return Model{
		messages:   []string{},
		picker:     nil,
		textInput:  ti,
		loading:    false,
		err:        "",
		spinner:    s,
		userStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("5")),
		astroStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("2")),
		sendMessage: func(input api.Input) tea.Cmd {
			return func() tea.Msg {
				output, err := sendMessage(input)
				if err != nil {
					return err.(tea.Msg)
				}

				return output.(tea.Msg)
			}
		},
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit

		case "enter":
			focused := m.focused()
			switch focused {
			case focusedLoading:
				//noop
				return m, nil
			case focusedPicker:
				m.loading = true
				m.err = ""
				return m, tea.Batch(
					m.sendMessage(api.Input{
						Message:   m.picker.SelectedItem().(listElement).value,
						SessionId: m.session,
					}),
					m.spinner.Tick,
				)
			case focusedInput:
				m.loading = true
				m.err = ""
				return m, tea.Batch(
					m.sendMessage(api.Input{
						Message:   m.textInput.Value(),
						SessionId: m.session,
					}),
					m.spinner.Tick,
				)
			}
		}

	case api.Output:
		sessionId := msg.SessionId()
		m.session = &sessionId
		m.loading = false
		switch m.focused() {
		case focusedInput:
			m.messages = append(m.messages, m.userStyle.Render("User:")+" "+m.textInput.Value())
			break
		case focusedPicker:
			selected := m.picker.SelectedItem().(listElement)
			m.messages = append(m.messages, "User: "+selected.text+"("+selected.value+")")
			break
		}

		m.textInput.SetValue("")
		m.picker = nil
		for _, message := range msg.Messages() {
			if message.Text != nil {
				content := m.astroStyle.Render("Astro:") + " " + *message.Text
				m.messages = append(m.messages, content)
			}
			if message.Options != nil {
				// Add a text entry
				content := m.astroStyle.Render("Astro:") + " ("
				var textItems []string
				for _, option := range message.Options {
					textItems = append(textItems, option.Text+"["+option.Value+"]")
				}

				content += strings.Join(textItems, ",")
				content += ")"
				m.messages = append(m.messages, content)

				var items []list.Item
				for _, option := range message.Options {
					items = append(items, listElement{
						text:  option.Text,
						value: option.Value,
					})
				}
				picker := list.New(items, itemDelegate{}, 75, 14)
				if message.Text != nil {
					picker.Title = *message.Text
				} else {
					picker.Title = "Pick one"
				}
				m.picker = &picker
			} else {
				m.picker = nil
			}

			if message.Command != nil {
				m.messages = append(m.messages, "/"+*message.Command)
			}

			if message.Pause > 0 {
				m.messages = append(m.messages, "<pause>")
			}

		}

	case error:
		m.loading = false
		m.err = fmt.Sprintf("Failed to send the message - please try again. (%v)", msg.Error())
	}

	focused := m.focused()
	switch focused {
	case focusedLoading:
		m.spinner, cmd = m.spinner.Update(msg)
		break
	case focusedInput:
		m.textInput, cmd = m.textInput.Update(msg)
		break
	case focusedPicker:
		var updatedPicker list.Model
		updatedPicker, cmd = (*m.picker).Update(msg)
		m.picker = &updatedPicker
		break
	}

	return m, cmd
}

func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

func (m Model) View() string {
	// The header
	s := "Astro command line utility\n\n"

	// Iterate over our choices
	for _, message := range m.messages {
		// Render the row
		s += fmt.Sprintf("%s\n", message)
	}

	s += "\n\n"

	switch m.focused() {
	case focusedPicker:
		s += m.picker.View()
		break
	case focusedInput:
		s += m.textInput.View()
		break
	}

	if m.err != "" {
		s += "\n" + m.err
	}

	if m.loading {
		s += "\n" + m.spinner.View() + " Loading..."
	}

	// The footer
	s += "\n\nPress ctrl+c to quit.\n"

	// Send the UI for rendering
	return s
}
