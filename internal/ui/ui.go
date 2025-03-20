package ui

import (
	"fmt"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	"io"
	"strings"
	"virtual-assistant-cli/internal/api"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const focusInput = 1
const focusMessages = 2

type Model struct {
	ready bool

	loading bool
	focus   int

	viewportBuffer  string
	messageViewport viewport.Model
	textArea        textarea.Model
	picker          *list.Model
	err             string
	spinner         spinner.Model

	sendMessage func(input api.Input) tea.Cmd
	session     *string

	screenWidth  int
	screenHeight int
}

var (
	userStyle                 = lipgloss.NewStyle().Foreground(lipgloss.Color("5"))
	astroStyle                = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	debugOutputStyle          = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("#FFCC00"))
	itemStyle                 = lipgloss.NewStyle().PaddingLeft(4)
	listSelectedItemStyle     = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("170"))
	listBlurSelectedItemStyle = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("110"))
	blurMessageViewport       = lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder())
	focusedMessageViewport    = lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("170"))
)

type listElement struct {
	text  string
	value string
	id    *string
}

func (i listElement) FilterValue() string { return "" }

type itemDelegate struct {
	Focused bool
}

func (d itemDelegate) Height() int                             { return 1 }
func (d itemDelegate) Spacing() int                            { return 0 }
func (d itemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(listElement)
	if !ok {
		return
	}

	str := fmt.Sprintf("%d. %s", index+1, i.text)
	if i.id != nil {
		str += "[" + *i.id + "]"
	}

	fn := itemStyle.Render
	if index == m.Index() {
		fn = func(s ...string) string {
			style := listSelectedItemStyle
			if !d.Focused {
				style = listBlurSelectedItemStyle
			}
			return style.Render("> " + strings.Join(s, " "))
		}
	}

	fmt.Fprint(w, fn(str))
}

func CreateModel(sendMessage func(api.Input) (api.Output, error)) Model {
	textArea := textarea.New()
	textArea.Placeholder = "Send a message..."
	textArea.Focus()
	textArea.SetWidth(80)
	textArea.SetHeight(3)
	textArea.ShowLineNumbers = false

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return Model{
		focus:          focusInput,
		viewportBuffer: "",
		picker:         nil,
		textArea:       textArea,
		loading:        false,
		err:            "",
		spinner:        s,
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

func (m Model) topView() string {
	return "Virtual assistant\n\n"
}

func (m Model) bottomView() string {
	s := "\n\n"

	if m.picker != nil {
		s += m.picker.View()
	} else {
		s += m.textArea.View()
	}

	if m.err != "" {
		s += "\n" + m.err
	}

	if m.loading {
		s += "\n" + m.spinner.View() + " Loading..."
	}

	// The footer
	s += "\n\nPress: ctrl+c to quit and Tab to switch focus.\n"

	return s
}

func updateSizes(m *Model) {
	topHeight := lipgloss.Height(m.topView())
	bottomHeight := lipgloss.Height(m.bottomView())
	verticalMarginHeight := bottomHeight + topHeight
	width := m.screenWidth - 2
	if !m.ready {
		m.messageViewport = viewport.New(width, m.screenHeight-verticalMarginHeight)
		m.messageViewport.YPosition = topHeight
		m.ready = true
	} else {
		m.messageViewport.Width = width
		m.messageViewport.Height = m.screenHeight - verticalMarginHeight
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.screenWidth = msg.Width
		m.screenHeight = msg.Height
		updateSizes(&m)

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit

		case "tab":
			if m.focus == focusMessages {
				m.focus = focusInput
			} else {
				m.focus = focusMessages
			}

		case "enter":
			if m.loading {
				return m, nil
			}

			if m.focus == focusInput {
				if m.picker != nil {
					if m.picker.SelectedItem().(listElement).value == "" {
						m.picker = nil
						break
					}

					m.loading = true
					m.err = ""
					var optionId string
					if m.picker.SelectedItem().(listElement).id != nil {
						optionId = *m.picker.SelectedItem().(listElement).id
					}
					return m, tea.Batch(
						m.sendMessage(api.Input{
							Message:   m.picker.SelectedItem().(listElement).value,
							SessionId: m.session,
							OptionId:  optionId,
						}),
						m.spinner.Tick,
					)
				} else {
					m.loading = true
					m.err = ""
					return m, tea.Batch(
						m.sendMessage(api.Input{
							Message:   m.textArea.Value(),
							SessionId: m.session,
						}),
						m.spinner.Tick,
					)
				}
			}
			break
		}

	case api.Output:
		sessionId := msg.SessionId()
		m.session = &sessionId
		m.loading = false
		if m.picker != nil {
			selected := m.picker.SelectedItem().(listElement)
			m.viewportBuffer += userStyle.Render("User:") + " " + selected.text + "(" + selected.value + ")"
			if selected.id != nil {
				m.viewportBuffer += "[id=" + *selected.id + "]"
			}
			m.viewportBuffer += "\n"
		} else {
			m.viewportBuffer += userStyle.Render("User:") + " " + m.textArea.Value() + "\n"
		}

		m.textArea.SetValue("")

		m.picker = nil
		for _, message := range msg.Messages() {
			if message.Text != nil {
				content := astroStyle.Render("Astro:") + " " + *message.Text
				m.viewportBuffer += content + "\n"
			}
			if message.Options != nil {
				content := ""
				for _, option := range message.Options {
					content += astroStyle.Render("  - ") + option.Text + "(" + option.Value + ")\n"
				}

				m.viewportBuffer += content + "\n"

				var items []list.Item
				for _, option := range message.Options {
					items = append(items, listElement{
						text:  option.Text,
						value: option.Value,
						id:    option.OptionId,
					})
				}
				items = append(items, listElement{
					text:  "Type something",
					value: "",
					id:    nil,
				})

				picker := list.New(items, itemDelegate{Focused: true}, 75, 14)
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
				m.viewportBuffer += "/" + *message.Command + "\n"
			}

			if message.Pause > 0 {
				m.viewportBuffer += "<pause>\n"
			}

		}

		if msg.Debug() != "" {
			m.viewportBuffer += "\n" + debugOutputStyle.Render(msg.Debug()) + "\n\n"
		}

		m.messageViewport.SetContent(m.viewportBuffer)
		m.messageViewport.GotoBottom()
		updateSizes(&m)
		break

	case error:
		m.loading = false
		m.err = fmt.Sprintf("Failed to send the message - please try again. (%v)", msg.Error())
	}

	if m.loading {
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	if m.focus == focusInput {
		if m.picker != nil {
			var updatedPicker list.Model
			updatedPicker, cmd = (*m.picker).Update(msg)
			updatedPicker.SetDelegate(itemDelegate{Focused: true})
			cmds = append(cmds, cmd)
			m.picker = &updatedPicker
		} else {
			m.textArea.Focus()
		}
	} else {
		m.textArea.Blur()
		if m.picker != nil {
			m.picker.SetDelegate(itemDelegate{Focused: false})
		}
	}

	m.textArea, cmd = m.textArea.Update(msg)
	cmds = append(cmds, cmd)

	if m.focus == focusMessages {
		m.messageViewport, cmd = m.messageViewport.Update(msg)
		m.messageViewport.Style = focusedMessageViewport
		cmds = append(cmds, cmd)
	} else {
		m.messageViewport.Style = blurMessageViewport
	}

	return m, tea.Batch(cmds...)
}

func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

func (m Model) View() string {
	if !m.ready {
		return "\n Initializing..."
	}

	// The header
	s := m.topView()

	s += m.messageViewport.View()
	s += m.bottomView()

	// Send the UI for rendering
	return s
}
