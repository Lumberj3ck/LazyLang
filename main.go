package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	lazy_db "lazylang/db"
	"lazylang/piper"
	"lazylang/transriber"
	"lazylang/utils"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/tmc/langchaingo/chains"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/memory"
	"github.com/tmc/langchaingo/memory/sqlite3"
	"github.com/tmc/langchaingo/prompts"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// var docStyle = lipgloss.NewStyle().Margin(1, 2)
const (
	scrolloff = 2
)

var isAlpha = regexp.MustCompile(`[\p{L}]+`)

type model struct {
	DB               *sql.DB
	chats            list.Model
	llmChain         *ChatLLMChain
	viewport         viewport.Model
	content          string
	ready            bool
	recorder         *Recorder
	piperVoice       *piper.PiperVoice
	status           string
	focusColl        int
	focusRow         int
	fullWidth        int
	fullHeight       int
	cancelSpeak      context.CancelFunc
	wordsStore       *WordsStore
	config           Config
	transcriber      transriber.Transcriber
	downloadingModel bool
	showChatSessions bool
	initialTopic     string
}

func initialModel(propose bool, config Config) model {
	llm, err := NewLLM(
		WithBaseURL(config.CompletionProvider.BaseURL),
		WithToken(config.CompletionProvider.Token),
		WithModel(config.CompletionProvider.Model),
	)
	if err != nil {
		fmt.Printf("Error creating LLM: %v\n", err)
		os.Exit(1)
	}

	systemMsg := fmt.Sprintf("You are a %s teacher. Respond to the following question or statement in %s. Important: only give short answers to the questions!", config.Language, config.Language)
	prompt := prompts.NewChatPromptTemplate([]prompts.MessageFormatter{
		prompts.NewSystemMessagePromptTemplate(systemMsg, nil),
		prompts.MessagesPlaceholder{VariableName: "history"},
		prompts.NewHumanMessagePromptTemplate("{{.text}}", []string{"text"}),
	})

	llmChain := NewChatLLMChain(llm, prompt)
	DefaultSchema := []byte(lazy_db.DefaultSchema)

	persistedHistory := sqlite3.NewSqliteChatMessageHistory(sqlite3.WithDBAddress("chats.db"), sqlite3.WithSchema(DefaultSchema))
	session_id, err := lazy_db.CreateChatSession(persistedHistory.DB)
	if err != nil {
		fmt.Printf("Error creating LLM: Error creating sql schema: %v\n", err)
		os.Exit(1)
	}
	persistedHistory.Session = fmt.Sprintf("%d", session_id)

	var topic string
	if propose {
		topic, err = llms.GenerateFromSinglePrompt(context.Background(), llm,
			fmt.Sprintf("Propose a random conversation topic or question in %s. Choose from a wide variety of subjects: hobbies, travel, food, culture, work, dreams, nature, technology, sports, art, etc. Reply only with the topic or question itself, nothing else.", config.Language),
			llms.WithTemperature(2),
		)
		if err != nil {
			fmt.Println("Couldn't propose topic failed with: ", err.Error())
			os.Exit(1)
		}
		err = persistedHistory.AddAIMessage(context.Background(), topic)
		if err != nil {
			fmt.Println("Failed to add starting topic: ", err.Error())
			os.Exit(1)
		}
	}

	llmChain.Memory = memory.NewConversationBuffer(
		memory.WithChatHistory(persistedHistory),
		memory.WithReturnMessages(true),
	)
	piperVoice := piper.NewPiperVoice(piper.WithModel(config.TTSBackend.Voice), piper.WithLanguage(config.Language))

	var transcriber transriber.Transcriber
	switch config.STTBackend.Type {
	case HostedSTT:
		sttBaseURL := resolveHostedSTTBaseURL(config)
		sttToken := resolveHostedSTTToken(config)
		transcriber = transriber.NewOpenAITranscriber(sttBaseURL, sttToken, config.STTBackend.Model, config.Language)
	case LocalSTT:
		transcriber = transriber.NewWhispercppTranscriber(config.STTBackend.Model, config.Language)
	default:
		log.Fatalf("Error: Invalid STT backend %s", config.STTBackend.Type)
	}
	l := list.New(nil, list.NewDefaultDelegate(), 0, 0)

	l.SetShowTitle(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.SetShowStatusBar(false)
	return model{
		DB:           persistedHistory.DB,
		chats:        l,
		llmChain:     llmChain,
		recorder:     NewRecorder(),
		status:       "Ready",
		piperVoice:   piperVoice,
		wordsStore:   NewWordsStore(),
		config:       config,
		content:      "",
		transcriber:  transcriber,
		initialTopic: topic,
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func EmptyCmd() tea.Msg {
	return ""
}

type RecordingStarted struct{}
type TranscriptionReceived struct {
	transcription string
}
type StatusChanged struct {
	status string
}
type ReadyCompletion struct {
	completion string
	addContent bool
}

func GetLlmCompletion(text string, m model) tea.Cmd {
	return func() tea.Msg {
		output, err := chains.Call(context.Background(), m.llmChain, map[string]any{"text": text})
		if err != nil {
			log.Println(err)
			return StatusChanged{status: "Failed get completion"}
		}
		if output["text"] == nil {
			return StatusChanged{status: "No completion"}
		}
		return ReadyCompletion{completion: output["text"].(string), addContent: true}
	}
}

type DownloadPiperModel struct {
	model      string
	language   string
	completion string
}

func Speak(ctx context.Context, text string, m model) tea.Cmd {
	return func() tea.Msg {
		err := m.piperVoice.Speak(ctx, text)
		if err != nil {
			switch err := err.(type) {
			case piper.StoppedSpeaking:
				return ""
			case piper.ErrorModelNotFound:
				return DownloadPiperModel{model: err.Model, language: err.Language, completion: text}
			default:
				log.Printf("Error speaking: %v\n", err)
				return StatusChanged{status: "Failed to speak"}
			}
		}
		return StatusChanged{status: "Ready"}
	}
}

func HighlightFocusWord(wrapped_text string, focusRow int, focusWord int) string {
	var st strings.Builder
	for i, row := range strings.Split(strings.TrimSpace(wrapped_text), "\n") {
		if i == focusRow {
			for i, word := range strings.Split(strings.TrimSpace(row), " ") {
				if i == focusWord {
					log.Printf("FocusWord: %q %v", word, i)
					st.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render(word))
				} else {
					st.WriteString(word)
				}
				st.WriteRune(' ')
			}
		} else {
			st.WriteString(row)
		}
		st.WriteRune('\n')
	}

	return st.String()
}

type TranslationReceived struct {
	Word        string
	Translation string
}

func GetTranslation(word string, m model) tea.Cmd {
	return func() tea.Msg {
		lower := strings.ToLower(word)
		if translation, ok := m.wordsStore.Get(lower); ok {
			return TranslationReceived{Word: lower, Translation: translation}
		}
		translator, err := NewLLM(
			WithBaseURL(m.config.CompletionProvider.BaseURL),
			WithToken(m.config.CompletionProvider.Token),
			WithModel(m.config.CompletionProvider.Model),
		)
		if err != nil {
			log.Printf("Error creating translation model: %v", err)
			return StatusChanged{status: "Failed to translate"}
		}
		prompt := fmt.Sprintf("Translate the single word '%s' from %s to %s. Respond only with the translation.", word, m.config.Language, m.config.TargetTranslationLanguage)
		translation, err := llms.GenerateFromSinglePrompt(context.Background(), translator, prompt, llms.WithTemperature(0))
		if err != nil {
			log.Printf("Error calling translation provider: %v", err)
			return StatusChanged{status: "Failed to translate"}
		}
		translation = strings.TrimSpace(translation)
		if translation == "" {
			log.Printf("Translation provider returned empty result for word %s", word)
			return StatusChanged{status: "Failed to translate"}
		}
		return TranslationReceived{Word: lower, Translation: translation}
	}
}

func (m *model) UpdateStatus(status string) {
	if m.recorder.IsRecording() || m.piperVoice.IsSpeaking() {
		return
	}
	m.status = status
}

func setViewportContent(m *model, content string) {
	content = lipgloss.NewStyle().Width(m.viewport.Width).Render(content)
	m.viewport.SetContent(content)
}

type DownloadWhisperModel struct {
	model string
}

type FinishDownloadingWhisperModel struct{ err string }

func getWrappedContent(content string, width int) string {
	return lipgloss.NewStyle().Width(width).Render(content)
}

func GetTranscription(m model) tea.Cmd {
	return func() tea.Msg {
		transcription, err := m.transcriber.Transcribe(m.recorder.Content)
		if errors.Is(err, transriber.ErrNoModel) {
			log.Println("DOwnloading model")
			return DownloadWhisperModel{model: m.config.STTBackend.Model}
		}
		log.Println(transcription)
		if err != nil {
			log.Printf("Error transcribing audio: %v\n", err)
			return EmptyCmd
		}
		return TranscriptionReceived{transcription: transcription}
	}
}

func StartDownloadWhisperModel(m model, msg DownloadWhisperModel, downloadReport chan int64) tea.Cmd {
	return func() tea.Msg {
		tr, ok := m.transcriber.(*transriber.WhispercppTranscriber)
		if !ok {
			log.Println("Error casting transcriber to WhispercppTranscriber")
			return ""
		}
		err := tr.DownloadModel(msg.model, downloadReport)
		if err == utils.DownloadCanceledErr {
			return ""
		}
		if err != nil {
			log.Printf("Error downloading model: %v", err)
			return FinishDownloadingWhisperModel{err: "Failed to download whisper model"}
		}
		return FinishDownloadingWhisperModel{}
	}
}

type DownloadReportReceived struct {
	pct            int64
	modelType      string // STT or TTS
	downloadReport chan int64
}

func DownloadReport(downloadReport chan int64, modelType string) tea.Cmd {
	return func() tea.Msg {
		for pct := range downloadReport {
			return DownloadReportReceived{pct: pct, modelType: modelType, downloadReport: downloadReport}
		}
		return ""
	}
}

func StartDownloadPiperModel(m model, msg DownloadPiperModel, downloadReport chan int64) tea.Cmd {
	return func() tea.Msg {
		err := m.piperVoice.DownloadVoice(msg.language, msg.model, downloadReport)
		if err == utils.DownloadCanceledErr {
			return ""
		}
		if err != nil {
			return StatusChanged{status: "Failed to download model"}
		}
		return ReadyCompletion{completion: msg.completion, addContent: false}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var initialTopicCmd tea.Cmd
	switch msg := msg.(type) {
	case DownloadPiperModel:
		m.UpdateStatus("Downloading tts model")
		downloadReport := make(chan int64, 10)
		return m, tea.Batch(StartDownloadPiperModel(m, msg, downloadReport), DownloadReport(downloadReport, "TTS"))
	case DownloadWhisperModel:
		m.downloadingModel = true
		m.UpdateStatus("Downloading model")

		downloadReport := make(chan int64, 10)
		return m, tea.Batch(StartDownloadWhisperModel(m, msg, downloadReport), DownloadReport(downloadReport, "STT"))
	case DownloadReportReceived:
		m.UpdateStatus(fmt.Sprintf("Downloading %s model: %d%%", msg.modelType, msg.pct))
		return m, DownloadReport(msg.downloadReport, msg.modelType)
	case FinishDownloadingWhisperModel:
		m.downloadingModel = false
		if msg.err != "" {
			m.UpdateStatus(msg.err)
			return m, EmptyCmd
		}

		m.UpdateStatus("Model downloaded")
		return m, GetTranscription(m)

	case StatusChanged:
		m.UpdateStatus(msg.status)
	case ReadyCompletion:
		if msg.addContent {
			sanitisedCompletion := strings.ReplaceAll(msg.completion, "\n\n", "\n")
			m.content = fmt.Sprintf("%sAI: %s \n", m.content, sanitisedCompletion)
			highlightedCompletion := HighlightFocusWord(m.content, m.focusRow, m.focusColl)
			setViewportContent(&m, highlightedCompletion)
			m.viewport.GotoBottom()

			// move cursor into view port if scrolled too much
			if m.focusRow < m.viewport.YOffset {
				m.focusRow = m.viewport.YOffset
				highlightedCompletion := HighlightFocusWord(m.content, m.focusRow, m.focusColl)
				setViewportContent(&m, highlightedCompletion)
			}
		}

		m.UpdateStatus("Speaking")

		ctx, cancel := context.WithCancel(context.Background())
		m.cancelSpeak = cancel
		return m, Speak(ctx, msg.completion, m)

	case TranscriptionReceived:
		m.content = fmt.Sprintf("%sYou:%s \n", m.content, msg.transcription)
		highlighted := HighlightFocusWord(m.content, m.focusRow, m.focusColl)
		setViewportContent(&m, highlighted)
		m.viewport.GotoBottom()

		// move cursor into view port if scrolled too much
		if m.focusRow < m.viewport.YOffset {
			m.focusRow = m.viewport.YOffset
			highlightedCompletion := HighlightFocusWord(m.content, m.focusRow, m.focusColl)
			setViewportContent(&m, highlightedCompletion)
		}
		return m, GetLlmCompletion(msg.transcription, m)

	case TranslationReceived:
		m.wordsStore.Add(msg.Word, msg.Translation)

	case tea.KeyMsg:
		switch k := msg.String(); k {
		case "enter":
			if m.showChatSessions {
				s := m.chats.SelectedItem()
				c, ok := s.(lazy_db.Chat)
				if !ok {
					break
				}

				DefaultSchema := []byte(lazy_db.DefaultSchema)
				persistendHistory := sqlite3.NewSqliteChatMessageHistory(sqlite3.WithDBAddress("chats.db"), sqlite3.WithSchema(DefaultSchema))
				persistendHistory.Session = strconv.Itoa(c.GetId())
				msgs, err := persistendHistory.Messages(context.Background())

				if err != nil {
					log.Printf("Couldn't load message from session with id %d: %s", c.GetId(), err.Error())
					break
				}
				m.content = ""
				for _, msg := range msgs {
					switch msg.GetType() {
					case llms.ChatMessageTypeAI:
						m.content += fmt.Sprintf("AI: %s \n", msg.GetContent())
					case llms.ChatMessageTypeHuman:
						m.content += fmt.Sprintf("You: %s \n", msg.GetContent())
					}
				}
				m.llmChain.Memory = memory.NewConversationBuffer(
					memory.WithChatHistory(persistendHistory),
					memory.WithReturnMessages(true),
				)

				m.DB.Close()
				m.DB = persistendHistory.DB
				m.focusColl = 0
				m.focusRow = 0

				highlightedCompletion := HighlightFocusWord(m.content, m.focusRow, m.focusColl)
				setViewportContent(&m, highlightedCompletion)
				m.showChatSessions = false
				return m, EmptyCmd
			}
			selectedWord := m.getFocusedWord()
			clearedWord := isAlpha.FindString(selectedWord)
			if clearedWord == "" {
				m.UpdateStatus("Nothing to translate")
				return m, EmptyCmd
			}
			return m, GetTranslation(clearedWord, m)

		case "esc":
			if m.cancelSpeak != nil {
				m.cancelSpeak()
			}
			m.piperVoice.CancelDownload()
			t, ok := m.transcriber.(*transriber.WhispercppTranscriber)
			if ok {
				t.CancelDownload()
			}
			m.UpdateStatus("Ready")
			return m, EmptyCmd
		case "j":
			if m.showChatSessions {
				break
			}
			wrappedCompletion := getWrappedContent(m.content, m.viewport.Width)
			rows := strings.Split(strings.TrimSpace(wrappedCompletion), "\n")
			if len(rows) == 0 || len(wrappedCompletion) == 0 {
				break
			}
			if m.focusRow+1 >= len(rows) {
				break
			}
			m.focusRow++

			focusedRow := rows[m.focusRow]
			m.focusColl = min(max(len(strings.Split(strings.TrimSpace(focusedRow), " "))-1, 0), m.focusColl)

			highlightedCompletion := HighlightFocusWord(wrappedCompletion, m.focusRow, m.focusColl)
			setViewportContent(&m, highlightedCompletion)
			log.Printf("FocusWord j: %v %v", m.focusColl, m.focusRow)

			// If we're not at scrolloff, don't scroll
			visibleLines := m.viewport.VisibleLineCount()
			if (visibleLines+m.viewport.YOffset)-m.focusRow > scrolloff {
				return m, EmptyCmd
			}
		case "k":
			if m.showChatSessions {
				break
			}
			if m.focusRow-1 < 0 {
				break
			}
			m.focusRow--

			wrappedCompletion := getWrappedContent(m.content, m.viewport.Width)
			rows := strings.Split(strings.TrimSpace(wrappedCompletion), "\n")
			if len(rows) == 0 {
				break
			}

			focusedRow := rows[m.focusRow]
			m.focusColl = min(max(len(strings.Split(strings.TrimSpace(focusedRow), " "))-1, 0), m.focusColl)

			highlightedCompletion := HighlightFocusWord(wrappedCompletion, m.focusRow, m.focusColl)
			setViewportContent(&m, highlightedCompletion)

			// If we're not at scrolloff, don't scroll
			if m.focusRow-(m.viewport.YOffset-1) > scrolloff {
				return m, EmptyCmd
			}
		case "w":
			wrappedCompletion := getWrappedContent(m.content, m.viewport.Width)
			rows := strings.Split(strings.TrimSpace(wrappedCompletion), "\n")
			if len(rows) == 0 {
				break
			}

			focusedRow := rows[m.focusRow]
			if m.focusColl+1 >= len(strings.Split(focusedRow, " ")) && m.focusRow+1 >= len(rows) {
				break
			}

			if m.focusColl+1 >= len(strings.Split(strings.TrimSpace(focusedRow), " ")) {
				m.focusRow++
				m.focusColl = -1
			}

			m.focusColl++
			highlightedCompletion := HighlightFocusWord(wrappedCompletion, m.focusRow, m.focusColl)
			setViewportContent(&m, highlightedCompletion)

			// If we're not at scrolloff, don't scroll
			visibleLines := m.viewport.VisibleLineCount()
			if (visibleLines+m.viewport.YOffset)-m.focusRow > scrolloff {
				return m, EmptyCmd
			}
			m.viewport.ScrollDown(1)
		case "b":
			wrappedCompletion := getWrappedContent(m.content, m.viewport.Width)
			if m.focusColl-1 < 0 && m.focusRow-1 < 0 {
				break
			} else if m.focusColl-1 < 0 {
				m.focusRow = max(0, m.focusRow-1)
				rows := strings.Split(strings.TrimSpace(wrappedCompletion), "\n")
				focusedRow := strings.Split(strings.TrimSpace(rows[m.focusRow]), " ")
				m.focusColl = len(focusedRow)
			}

			m.focusColl--
			highlightedCompletion := HighlightFocusWord(wrappedCompletion, m.focusRow, m.focusColl)
			setViewportContent(&m, highlightedCompletion)

			// If we're not at scrolloff, don't scroll
			if m.focusRow-(m.viewport.YOffset-1) > scrolloff {
				return m, EmptyCmd
			}
			m.viewport.ScrollUp(1)
			return m, EmptyCmd
		case "ctrl+p":
			m.showChatSessions = !m.showChatSessions
			if m.showChatSessions {
				chats, err := lazy_db.GetActiveChats(m.DB)
				if err != nil {
					log.Println("Couldn't retrieve chats ", err.Error())
					break
				}
				items := make([]list.Item, len(chats))
				for i, c := range chats {
					items[i] = c
				}
				m.chats.SetItems(items)
			}
			return m, EmptyCmd
		case "ctrl+b":
			if m.downloadingModel {
				m.UpdateStatus("Please wait for the model to download")
				return m, EmptyCmd
			}

			if m.cancelSpeak != nil {
				m.cancelSpeak()
			}

			if time.Since(m.recorder.Stopped) < time.Second {
				return m, EmptyCmd
			}

			if m.recorder.IsRecording() {
				m.recorder.Stop()
				m.UpdateStatus("Ready")
				return m, GetTranscription(m)
			}

			m.UpdateStatus("Recording")
			return m, func() tea.Msg {
				m.recorder.Start()
				return ""
			}
		case "ctrl+c", "q":
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.fullWidth = msg.Width
		m.fullHeight = msg.Height
		headerHeight := lipgloss.Height(m.headerView()) + 1
		viewportWidth := msg.Width*3/4 + 1
		viewportHeight := msg.Height - headerHeight

		if !m.ready {
			viewport := viewport.New(viewportWidth, viewportHeight)
			viewport.YPosition = headerHeight
			viewport.SetContent(m.content)
			m.viewport = viewport
			m.chats.SetSize(msg.Width*3/4, msg.Height*3/4)

			m.ready = true
		} else {
			m.viewport.Width = viewportWidth
			m.viewport.Height = viewportHeight
		}
		highlightedCompletion := HighlightFocusWord(m.content, m.focusRow, m.focusColl)
		setViewportContent(&m, highlightedCompletion)
		if m.initialTopic != "" {
			topic := m.initialTopic
			m.initialTopic = ""
			initialTopicCmd = func() tea.Msg {
				return ReadyCompletion{completion: topic, addContent: true}
			}
		}
	}

	var cmds []tea.Cmd
	var cmd tea.Cmd

	m.viewport, cmd = m.viewport.Update(msg)
	m.chats, cmd = m.chats.Update(msg)

	cmds = append(cmds, cmd)
	if initialTopicCmd != nil {
		cmds = append(cmds, initialTopicCmd)
	}
	return m, tea.Batch(cmds...)
}

var titleStyle = func() lipgloss.Style {
	b := lipgloss.RoundedBorder()
	b.BottomRight = "┴"
	return lipgloss.NewStyle().BorderStyle(b).Padding(0, 1)
}()

func (m model) getFocusedWord() string {
	wrapped := strings.TrimSpace(getWrappedContent(m.content, m.viewport.Width))
	if wrapped == "" {
		return ""
	}
	rows := strings.Split(wrapped, "\n")
	if m.focusRow < 0 || m.focusRow >= len(rows) {
		return ""
	}
	words := strings.Split(strings.TrimSpace(rows[m.focusRow]), " ")
	if len(words) == 0 {
		return ""
	}
	if m.focusColl < 0 || m.focusColl >= len(words) {
		return ""
	}
	return words[m.focusColl]
}

func (m model) headerView() string {
	title := titleStyle.Render("LazyLang")

	blockLength := max(0, m.fullWidth-lipgloss.Width(title))

	line := strings.Repeat("─", blockLength)

	statusLength := max(0, blockLength-lipgloss.Width(m.status))
	statusLine := strings.Repeat(" ", statusLength) + m.status

	s := lipgloss.JoinVertical(lipgloss.Center, statusLine, line)

	return lipgloss.JoinHorizontal(lipgloss.Center, title, s)
}

func (m model) sidebarView() string {
	b := lipgloss.NewStyle().
		Height(m.viewport.Height).
		Width(m.fullWidth*1/4 - 1).
		Border(lipgloss.NormalBorder()).
		BorderLeft(true).
		BorderTop(false).
		BorderRight(false).
		BorderBottom(false)

	return b.Render(m.wordsStore.List())
}

func (m model) View() string {
	content := lipgloss.JoinHorizontal(lipgloss.Center, m.viewport.View(), m.sidebarView())
	if m.showChatSessions {
		popup := m.chats.View()

		// Center the popup over the background
		return lipgloss.Place(
			m.fullWidth, m.fullHeight,
			lipgloss.Center, lipgloss.Center,
			popup,
			lipgloss.WithWhitespaceChars(" "),
		)
	} else {
		return fmt.Sprintf("%s\n%s\n", m.headerView(), content)
	}

}

func main() {
	config, err := GetConfig()

	if err != nil {
		log.Fatalf("Error parsing config: %v", err)
	}

	proposeTopic := flag.Bool("propose", false, "When this flag is specified conversation topic will be proposed")
	flag.Parse()

	p := tea.NewProgram(
		initialModel(*proposeTopic, config),
		tea.WithAltScreen(),       // use the full size of the terminal in its "alternate screen buffer"
		tea.WithMouseCellMotion(), // turn on mouse support so we can track the mouse wheel
	)
	f, err := tea.LogToFile("tea.log", "")
	if err != nil {
		fmt.Println("could not run program:", err)
		os.Exit(1)
	}
	defer f.Close()

	m, err := p.Run()
	my := m.(model)
	if my.cancelSpeak != nil {
		my.cancelSpeak()
	}

	if err != nil {
		fmt.Println("could not run program:", err)
		os.Exit(1)
	}
}
