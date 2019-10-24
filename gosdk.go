package gosdk

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/briscola-as-a-service/game/card"
	"github.com/briscola-as-a-service/game/hand"
	"github.com/briscola-as-a-service/game/player"
)

var seededRand = rand.New(rand.NewSource(time.Now().UnixNano()))

type GamePlayer interface {
	Play(PlayEvent) card.Card
}

type SDK struct {
	sessionID      string
	host           string
	feed           string
	subscriptionID string
	gamePlayer     GamePlayer
}

type PlayEvent struct {
	Cards          player.Cards `json:"Cards"`
	Briscola       card.Card    `json:"Briscola"`
	CurrentHands   []hand.Hand  `json:"Hands"`
	Message        string       `json:"Message"`
	ActionRequired bool         `json:"ActionRequired"`
}

type event struct {
	Data      PlayEvent `json:"Data"`
	Timestamp int32     `json:"Timestamp"`
}
type genericResponse struct {
	SubscriptionID string   `json:"SubscriptionID"`
	Feeds          []string `json:"Feeds"`
	Error          bool     `json:"Error"`
	ErrorCode      int      `json:"ErrorCode"`
	Message        string   `json:"Message"`
	Events         []event  `json:"Events"`
}

func NewGame(sessionID string, host string, gamePlayer GamePlayer) *SDK {
	sdk := SDK{
		sessionID:  sessionID,
		host:       host,
		gamePlayer: gamePlayer,
	}
	return &sdk
}

func (sdk *SDK) Play() (int, error) {
	// 1. Connect with the server and receive the feed and the connection token
	startResponse, err := sdk.startRequest()
	if err != nil {
		log.Fatalln(err)
	}

	if startResponse.Error == true {
		return 0, errors.New(startResponse.Message)
	}

	if startResponse.SubscriptionID == "" {
		return 0, errors.New("no subscriptionID returned")
	}

	sdk.subscriptionID = startResponse.SubscriptionID

	// 2. Listen to the returned feed
	for true {
		gr, err := sdk.listenEvents()
		if err != nil {
			log.Fatalln(err)
		}

		// In case of timeout, reconnect
		if gr.ErrorCode == 408 {
			show("...")
			continue
		}

		var events []event
		if gr.Error == false {
			events = gr.Events
		}

		// Be sure there is only an ActionRequest, and if present play a card.
		// Logs all other requestes
		var actionEvent event
		var actionEventCount = 0
		for _, event := range events {
			data := event.Data
			if data.Message != "" {
				event.MessageLog()
			}
			if data.ActionRequired == true {
				actionEventCount++
				actionEvent = event
			}
		}

		// Action required
		if actionEventCount == 1 {
			// Send the event to the client library
			cardToPlay := sdk.gamePlayer.Play(actionEvent.Data)
			show(cardToPlay)
			// TODO:
			// Play the card on the decker.
			// Inform with a message in the bradcast feed
			// Select next player (if needed) and send to next player the new card
		} else {
			// TODO handle the error. It should never happen
		}

		// If there are more cards, reconnect
	}

	return 10, nil
}

func (sdk *SDK) startRequest() (genericResponse, error) {
	var gr genericResponse
	startRequest := sdk.host + "/start?sessionID=" + sdk.sessionID + "&type=TEST"

	httpResponse, err := http.Get(startRequest)
	if err != nil {
		return gr, err
	}

	body, err := ioutil.ReadAll(httpResponse.Body)
	if err != nil {
		return gr, err
	}

	gr, err = parseResponse(body)
	if err != nil {
		return gr, err
	}
	return gr, nil
}

func (sdk *SDK) listenEvents() (genericResponse, error) {
	var gr genericResponse
	playRequest := sdk.host + "/play?sessionID=" + sdk.sessionID + "&subscriptionID=" + sdk.subscriptionID // + "&card=" + card

	httpResponse, err := http.Get(playRequest)
	if err != nil {
		return gr, err
	}

	body, err := ioutil.ReadAll(httpResponse.Body)
	if err != nil {
		return gr, err
	}

	gr, err = parseResponse(body)
	if err != nil {
		return gr, err
	}

	return gr, nil
}

func parseResponse(stream []byte) (genericResponse, error) {
	var response genericResponse
	err := json.Unmarshal(stream, &response)
	if err != nil {
		return response, err
	}
	return response, nil
}

func show(i interface{}) {
	fmt.Printf("*** %+v\n", i)
}

func (e event) MessageLog() {
	fmt.Printf("<< %d: %s\n", e.Timestamp, e.Data.Message)
}
