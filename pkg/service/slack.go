package service

import (
	"fmt"

	"github.com/charmbracelet/log"
	"github.com/google/uuid"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
	"github.com/spf13/viper"
	"github.com/theapemachine/a2a-go/pkg/a2a"
	"github.com/theapemachine/a2a-go/pkg/catalog"
)

type SlackService struct {
	slackClient *slack.Client
	appToken    string
	botToken    string
	catalogURL  string
}

func NewSlackService(appToken, botToken string) *SlackService {
	v := viper.GetViper()
	catalogURL := v.GetString("endpoints.catalog")
	if catalogURL == "" {
		log.Warn("SlackService: endpoints.catalog is not set in viper config. Agent interactions will fail.")
	}

	return &SlackService{
		appToken:   appToken,
		botToken:   botToken,
		catalogURL: catalogURL,
	}
}

func (srv *SlackService) Run() error {
	api := slack.New(
		srv.botToken,
		slack.OptionDebug(true),
		slack.OptionAppLevelToken(srv.appToken),
	)

	client := socketmode.New(
		api,
		socketmode.OptionDebug(true),
	)

	socketmodeHandler := socketmode.NewSocketmodeHandler(client)

	socketmodeHandler.Handle(socketmode.EventTypeConnecting, middlewareConnecting)
	socketmodeHandler.Handle(socketmode.EventTypeConnectionError, middlewareConnectionError)
	socketmodeHandler.Handle(socketmode.EventTypeConnected, middlewareConnected)
	socketmodeHandler.Handle(socketmode.EventTypeHello, middlewareHello)
	socketmodeHandler.Handle(socketmode.EventTypeEventsAPI, middlewareEventsAPI)
	socketmodeHandler.HandleEvents(slackevents.AppMention, srv.middlewareAppMentionEvent)
	socketmodeHandler.Handle(socketmode.EventTypeInteractive, middlewareInteractive)
	socketmodeHandler.HandleInteraction(slack.InteractionTypeBlockActions, middlewareInteractionTypeBlockActions)
	socketmodeHandler.Handle(socketmode.EventTypeSlashCommand, middlewareSlashCommand)
	socketmodeHandler.HandleSlashCommand("/rocket", middlewareSlashCommand)

	return socketmodeHandler.RunEventLoop()
}

func middlewareConnecting(evt *socketmode.Event, client *socketmode.Client) {
	log.Info("Connecting to Slack with Socket Mode...")
}

func middlewareConnectionError(evt *socketmode.Event, client *socketmode.Client) {
	log.Error("Connection failed. Retrying later...")
}

func middlewareConnected(evt *socketmode.Event, client *socketmode.Client) {
	log.Info("Connected to Slack with Socket Mode.")
}

func middlewareHello(evt *socketmode.Event, client *socketmode.Client) {
	log.Info("Received a hello message. Howdy to you too.")
}

func middlewareEventsAPI(evt *socketmode.Event, client *socketmode.Client) {
	log.Info("middlewareEventsAPI")
	eventsAPIEvent, ok := evt.Data.(slackevents.EventsAPIEvent)
	if !ok {
		log.Error("Ignored %+v\n", evt)
		return
	}

	log.Info("Event received: %+v\n", eventsAPIEvent)

	client.Ack(*evt.Request)

	switch eventsAPIEvent.Type {
	case slackevents.CallbackEvent:
		innerEvent := eventsAPIEvent.InnerEvent
		switch ev := innerEvent.Data.(type) {
		case *slackevents.AppMentionEvent:
			log.Info("We have been mentioned in %v (via middlewareEventsAPI)", ev.Channel)
			_, _, err := client.Client.PostMessage(
				ev.Channel,
				slack.MsgOptionText("Yes, hello. (from EventsAPI handler)", false),
			)
			if err != nil {
				log.Error("failed posting message: %v", err)
			}
		case *slackevents.MemberJoinedChannelEvent:
			log.Info("user %q joined to channel %q", ev.User, ev.Channel)
		}
	default:
		client.Debugf("unsupported Events API event received")
	}
}

func (srv *SlackService) middlewareAppMentionEvent(evt *socketmode.Event, client *socketmode.Client) {
	eventsAPIEvent, ok := evt.Data.(slackevents.EventsAPIEvent)
	if !ok {
		log.Error("Ignored %+v\n", evt)
		return
	}

	client.Ack(*evt.Request)

	ev, ok := eventsAPIEvent.InnerEvent.Data.(*slackevents.AppMentionEvent)
	if !ok {
		log.Error("Ignored: Expected AppMentionEvent, got: %+v\n", eventsAPIEvent.InnerEvent.Data)
		return
	}

	log.Info("AppMentionEvent: User %s mentioned bot in channel %s with text: '%s'", ev.User, ev.Channel, ev.Text)

	if srv.catalogURL == "" {
		log.Error("SlackService: catalogURL is not configured. Cannot interact with agent.")
		_, _, err := client.Client.PostMessage(ev.Channel, slack.MsgOptionText("I'm sorry, but I'm not properly configured to process your request right now.", false))
		if err != nil {
			log.Error("failed posting configuration error message to Slack: %v", err)
		}
		return
	}

	catalogClient := catalog.NewCatalogClient(srv.catalogURL)
	agents, err := catalogClient.GetAgents()
	if err != nil {
		log.Error("Failed to get agents from catalog: %v", err)
		errMsg := fmt.Sprintf("There was a problem reaching my knowledge base (catalog error): %s", err.Error())
		_, _, postErr := client.Client.PostMessage(ev.Channel, slack.MsgOptionText(errMsg, false))
		if postErr != nil {
			log.Error("failed posting catalog error message to Slack: %v", postErr)
		}
		return
	}

	var uiAgentCard *a2a.AgentCard
	for i := range agents {
		if agents[i].Name == "User Interface Agent" {
			uiAgentCard = &agents[i]
			break
		}
	}

	if uiAgentCard == nil {
		log.Error("'User Interface Agent' not found in catalog.")
		_, _, postErr := client.Client.PostMessage(ev.Channel, slack.MsgOptionText("I couldn't find the right internal component (UI agent) to handle your request.", false))
		if postErr != nil {
			log.Error("failed posting UI agent not found message to Slack: %v", postErr)
		}
		return
	}

	log.Info("Found User Interface Agent: %s at %s", uiAgentCard.Name, uiAgentCard.URL)
	agentClient := a2a.NewClient(uiAgentCard.URL)

	userMessageText := ev.Text

	messageToAgent := a2a.NewTextMessage("user", userMessageText)
	messageToAgent.Metadata = map[string]any{
		"origin":         "slack",
		"slackChannelID": ev.Channel,
		"slackUserID":    ev.User,
		"slackEventTS":   ev.EventTimeStamp,
	}

	taskID := uuid.New().String()
	sessionID := uuid.New().String()

	log.Info("Sending task to UI Agent", "taskID", taskID, "sessionID", sessionID, "message", userMessageText)

	jsonRpcResponse, taskErr := agentClient.SendTask(a2a.TaskSendParams{
		ID:        taskID,
		SessionID: sessionID,
		Message:   *messageToAgent,
	})

	if taskErr != nil {
		log.Error("Error sending task to UI Agent (client error): %v", taskErr)
		errMsg := fmt.Sprintf("I encountered an issue while communicating with my internal agent: %s", taskErr.Error())
		_, _, postErr := client.Client.PostMessage(ev.Channel, slack.MsgOptionText(errMsg, false))
		if postErr != nil {
			log.Error("failed posting task client error message to Slack: %v", postErr)
		}
		return
	}

	if jsonRpcResponse.Error != nil {
		log.Error("Error sending task to UI Agent (RPC error): Code %d, Message: %s", jsonRpcResponse.Error.Code, jsonRpcResponse.Error.Message)
		errMsg := fmt.Sprintf("I encountered an RPC error while processing your request: %s (Code: %d)", jsonRpcResponse.Error.Message, jsonRpcResponse.Error.Code)
		_, _, postErr := client.Client.PostMessage(ev.Channel, slack.MsgOptionText(errMsg, false))
		if postErr != nil {
			log.Error("failed posting task RPC error message to Slack: %v", postErr)
		}
		return
	}

	actualTask, ok := jsonRpcResponse.Result.(*a2a.Task)
	if !ok || actualTask == nil {
		log.Error("Failed to assert jsonRpcResponse.Result to *a2a.Task or result is nil. Actual type: %T", jsonRpcResponse.Result)
		errMsg := "I received an unexpected response format from my internal agent."
		_, _, postErr := client.Client.PostMessage(ev.Channel, slack.MsgOptionText(errMsg, false))
		if postErr != nil {
			log.Error("failed posting type assertion error message to Slack: %v", postErr)
		}
		return
	}

	log.Info("Task completed by UI Agent", "taskID", actualTask.ID, "status", actualTask.Status.State)

	var agentResponseText string
	for i := len(actualTask.History) - 1; i >= 0; i-- {
		msg := actualTask.History[i]
		if msg.Role == uiAgentCard.Name {
			if len(msg.Parts) > 0 {
				agentResponseText = msg.Parts[0].Text
			} else {
				agentResponseText = msg.String()
			}
			break
		}
	}

	if agentResponseText == "" {
		if actualTask.Status.Message != nil &&
			len(actualTask.Status.Message.Parts) > 0 &&
			actualTask.Status.Message.Parts[0].Text != "" &&
			actualTask.Status.Message.Role == uiAgentCard.Name {
			agentResponseText = actualTask.Status.Message.Parts[0].Text
		} else {
			agentResponseText = "I've processed your request."
			if actualTask.Status.State == a2a.TaskStateCompleted {
				agentResponseText = "Request processed successfully."
			}
			log.Warn("No specific response text found in task history or status from UI agent. TaskID: %s. Status: %s. Using generic reply.", actualTask.ID, actualTask.Status.State)
		}
	}

	log.Info("Sending response to Slack channel %s: %s", ev.Channel, agentResponseText)
	_, _, postErr := client.Client.PostMessage(ev.Channel, slack.MsgOptionText(agentResponseText, false))
	if postErr != nil {
		log.Error("failed posting agent response message to Slack: %v", postErr)
	}
}

func middlewareInteractive(evt *socketmode.Event, client *socketmode.Client) {
	callback, ok := evt.Data.(slack.InteractionCallback)
	if !ok {
		log.Error("Ignored %+v\n", evt)
		return
	}

	log.Info("Interaction received: %+v\n", callback)

	var payload interface{}

	switch callback.Type {
	case slack.InteractionTypeBlockActions:
	case slack.InteractionTypeShortcut:
	case slack.InteractionTypeViewSubmission:
	case slack.InteractionTypeDialogSubmission:
	default:

	}

	client.Ack(*evt.Request, payload)
}

func middlewareInteractionTypeBlockActions(
	evt *socketmode.Event,
	client *socketmode.Client,
) {
	client.Debugf("button clicked!")
}

func middlewareSlashCommand(
	evt *socketmode.Event,
	client *socketmode.Client,
) {
	cmd, ok := evt.Data.(slack.SlashCommand)
	if !ok {
		log.Error("Ignored %+v\n", evt)
		return
	}

	client.Debugf("Slash command received: %+v", cmd)

	payload := map[string]interface{}{
		"blocks": []slack.Block{
			slack.NewSectionBlock(
				&slack.TextBlockObject{
					Type: slack.MarkdownType,
					Text: "foo",
				},
				nil,
				slack.NewAccessory(
					slack.NewButtonBlockElement(
						"",
						"somevalue",
						&slack.TextBlockObject{
							Type: slack.PlainTextType,
							Text: "bar",
						},
					),
				),
			),
		}}
	client.Ack(*evt.Request, payload)
}
