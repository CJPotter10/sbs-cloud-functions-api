package cloudfunctions

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"
	"sync"

	"github.com/CJPotter10/sbs-cloud-functions-api/utils"
)

type Score struct {
	DST        float64 `json:"DST"`
	QB         float64 `json:"QB"`
	RB         float64 `json:"RB"`
	RB2        float64 `json:"RB2"`
	TE         float64 `json:"TE"`
	WR         float64 `json:"WR"`
	WR2        float64 `json:"WR2"`
	GameStatus string  `json:"GameStatus"`
	Team       string  `json:"Team"`
}

type Scores struct {
	FantasyPoints []Score `json:"FantasyPoints"`
}

type RosterPlayer struct {
	Team        string `json:"team"`
	PlayerId    string `json:"playerId"`
	DisplayName string `json:"displayName"`
}

type Roster struct {
	DST []RosterPlayer `json:"DST"`
	QB  []RosterPlayer `json:"QB"`
	RB  []RosterPlayer `json:"RB"`
	TE  []RosterPlayer `json:"TE"`
	WR  []RosterPlayer `json:"WR"`
}

type Prizes struct {
	ETH float64 `json:"ETH"`
}

type DraftToken struct {
	Roster            *Roster `json:"roster"`
	DraftType         string  `json:"_draftType"`
	CardId            string  `json:"_cardId"`
	ImageUrl          string  `json:"_imageUrl"`
	Level             string  `json:"_level"`
	OwnerId           string  `json:"_ownerId"`
	LeagueId          string  `json:"_leagueId"`
	LeagueDisplayName string  `json:"_leagueDisplayName"`
	Rank              string  `json:"_rank"`
	LeagueRank        string  `json:"_leagueRank"`
	WeekScore         string  `json:"_weekScore"`
	SeasonScore       string  `json:"_seasonScore"`
	Prizes            Prizes  `json:"prizes"`
}

type ScoreObject struct {
	PlayerId                   string  `json:"playerId"`
	PrevWeekSeasonContribution float64 `json:"prevWeekSeasonContribution"`
	ScoreSeason                float64 `json:"scoreSeason"`
	ScoreWeek                  float64 `json:"scoreWeek"`
	IsUsedInCardScore          bool    `json:"isUsedInCardScore"`
	Team                       string  `json:"team"`
	Position                   string  `json:"position"`
}

type ScoreRoster struct {
	DST []ScoreObject `json:"DST"`
	QB  []ScoreObject `json:"QB"`
	RB  []ScoreObject `json:"RB"`
	TE  []ScoreObject `json:"TE"`
	WR  []ScoreObject `json:"WR"`
}

type CardScores struct {
	CardId              string      `json:"_cardId"`
	Roster              ScoreRoster `json:"roster"`
	ScoreWeek           float64     `json:"scoreWeek"`
	ScoreSeason         float64     `json:"scoreSeason"`
	PrevWeekSeasonScore float64     `json:"prevWeekSeasonScore"`
}

func sortPlayerArray(players []ScoreObject) []ScoreObject {
	for i := 0; i <= len(players); i++ {
		for j := 1 + i; j < len(players); j++ {
			if players[i].ScoreWeek < players[j].ScoreWeek {
				intermediate := players[i]
				players[i] = players[j]
				players[j] = intermediate
			}
		}
	}

	return players
}

func updatePlayerObjectIfScoreCounts(obj ScoreObject) ScoreObject {
	obj.ScoreSeason = math.Round(float64(obj.PrevWeekSeasonContribution+obj.ScoreWeek)*100) / 100
	obj.IsUsedInCardScore = true

	return obj
}

func updatePlayerObjectIfScoreDoesNotCount(obj ScoreObject) ScoreObject {
	obj.ScoreSeason = math.Round(float64(obj.PrevWeekSeasonContribution)*100) / 100
	obj.IsUsedInCardScore = false

	return obj
}

func calculateSeasonScoreFromSortedRoster(card CardScores, flex ScoreObject) CardScores {
	roster := card.Roster
	card.ScoreWeek = math.Round(float64(roster.DST[0].ScoreWeek+roster.QB[0].ScoreWeek+roster.RB[0].ScoreWeek+roster.RB[1].ScoreWeek+roster.TE[0].ScoreWeek+roster.WR[0].ScoreWeek+roster.WR[1].ScoreWeek+flex.ScoreWeek)*100) / 100

	roster.DST[0] = updatePlayerObjectIfScoreCounts(roster.DST[0])
	roster.QB[0] = updatePlayerObjectIfScoreCounts(roster.QB[0])
	roster.RB[0] = updatePlayerObjectIfScoreCounts(roster.RB[0])
	roster.RB[1] = updatePlayerObjectIfScoreCounts(roster.RB[1])
	roster.TE[0] = updatePlayerObjectIfScoreCounts(roster.TE[0])
	roster.WR[0] = updatePlayerObjectIfScoreCounts(roster.WR[0])
	roster.WR[1] = updatePlayerObjectIfScoreCounts(roster.WR[1])

	for i := 1; i < len(roster.DST); i++ {
		roster.DST[i] = updatePlayerObjectIfScoreDoesNotCount(roster.DST[i])
	}

	for i := 1; i < len(roster.QB); i++ {
		roster.QB[i] = updatePlayerObjectIfScoreDoesNotCount(roster.QB[i])
	}

	if flex.Position == "RB1" || flex.Position == "RB2" {
		roster.RB[2] = updatePlayerObjectIfScoreCounts(roster.RB[2])
		for i := 3; i < len(roster.RB); i++ {
			roster.RB[i] = updatePlayerObjectIfScoreDoesNotCount(roster.RB[i])
		}
	} else {
		for i := 2; i < len(roster.RB); i++ {
			roster.RB[i] = updatePlayerObjectIfScoreDoesNotCount(roster.RB[i])
		}
	}

	if flex.Position == "TE" {
		roster.TE[1] = updatePlayerObjectIfScoreCounts(roster.TE[1])
		for i := 2; i < len(roster.TE); i++ {
			roster.TE[i] = updatePlayerObjectIfScoreDoesNotCount(roster.TE[i])
		}
	} else {
		for i := 1; i < len(roster.TE); i++ {
			roster.TE[i] = updatePlayerObjectIfScoreDoesNotCount(roster.TE[i])
		}
	}

	if flex.Position == "WR1" || flex.Position == "WR2" {
		roster.WR[2] = updatePlayerObjectIfScoreCounts(roster.WR[2])
		for i := 3; i < len(roster.WR); i++ {
			roster.WR[i] = updatePlayerObjectIfScoreDoesNotCount(roster.WR[i])
		}
	} else {
		for i := 2; i < len(roster.WR); i++ {
			roster.WR[i] = updatePlayerObjectIfScoreDoesNotCount(roster.WR[i])
		}
	}

	card.Roster = roster
	card.ScoreSeason = card.PrevWeekSeasonScore + card.ScoreWeek

	return card
}

func (s Scores) ScoreCards(token *DraftToken, gameweek string, wg *sync.WaitGroup, ticketQueue chan struct{}) {
	defer func() {
		<-ticketQueue
		wg.Done()
	}()
	scoresMap := make(map[string]Score)
	for i := 0; i < len(s.FantasyPoints); i++ {
		scoresMap[s.FantasyPoints[i].Team] = s.FantasyPoints[i]
	}

	var cardScores CardScores
	err := utils.Db.ReadDocument(fmt.Sprintf("drafts/%s/scores/%s/cards", token.LeagueId, gameweek), token.CardId, &cardScores)
	if err != nil {
		fmt.Println("Error reading card scores: ", err)
		return
	}

	for i := 0; i < len(cardScores.Roster.DST); i++ {
		cardScores.Roster.DST[i].ScoreWeek = scoresMap[cardScores.Roster.DST[i].Team].DST
	}

	cardScores.Roster.DST = sortPlayerArray(cardScores.Roster.DST)

	for i := 0; i < len(cardScores.Roster.QB); i++ {
		cardScores.Roster.QB[i].ScoreWeek = scoresMap[cardScores.Roster.QB[i].Team].QB
	}
	cardScores.Roster.QB = sortPlayerArray(cardScores.Roster.QB)

	for i := 0; i < len(cardScores.Roster.RB); i++ {
		if res := strings.Split(cardScores.Roster.RB[i].PlayerId, "-"); res[len(res)-1] == "RB2" {
			cardScores.Roster.RB[i].ScoreWeek = scoresMap[cardScores.Roster.RB[i].Team].RB2
		} else {
			cardScores.Roster.RB[i].ScoreWeek = scoresMap[cardScores.Roster.RB[i].Team].RB
		}
	}

	cardScores.Roster.RB = sortPlayerArray(cardScores.Roster.RB)

	for i := 0; i < len(cardScores.Roster.TE); i++ {
		cardScores.Roster.TE[i].ScoreWeek = scoresMap[cardScores.Roster.TE[i].Team].TE
	}

	cardScores.Roster.TE = sortPlayerArray(cardScores.Roster.TE)

	for i := 0; i < len(cardScores.Roster.WR); i++ {
		if res := strings.Split(cardScores.Roster.WR[i].PlayerId, "-"); res[len(res)-1] == "WR2" {
			cardScores.Roster.WR[i].ScoreWeek = scoresMap[cardScores.Roster.WR[i].Team].WR2
		} else {
			cardScores.Roster.WR[i].ScoreWeek = scoresMap[cardScores.Roster.WR[i].Team].WR
		}
	}

	cardScores.Roster.WR = sortPlayerArray(cardScores.Roster.WR)

	flexArray := make([]ScoreObject, 0)

	for i := 2; i < len(cardScores.Roster.RB); i++ {
		flexArray = append(flexArray, cardScores.Roster.RB[i])
	}
	for i := 1; i < len(cardScores.Roster.TE); i++ {
		flexArray = append(flexArray, cardScores.Roster.TE[i])
	}
	for i := 2; i < len(cardScores.Roster.WR); i++ {
		flexArray = append(flexArray, cardScores.Roster.WR[i])
	}

	sortedFlex := sortPlayerArray(flexArray)
	flexPlayer := sortedFlex[0]

	cardScores = calculateSeasonScoreFromSortedRoster(cardScores, flexPlayer)

	err = utils.Db.CreateOrUpdateDocument(fmt.Sprintf("drafts/%s/scores/%s/cards", token.LeagueId, gameweek), token.CardId, cardScores)
	if err != nil {
		fmt.Println("Error updating score for card: ", err)
		return
	}

	fmt.Println("finished scoring card ", token.CardId)
}

func ScoreDraftTokens(gameweek string, scores Scores) error {
	tokensResponse, err := utils.Db.Client.Collection("draftTokens").Documents(context.Background()).GetAll()
	if err != nil {
		fmt.Println("Error reading all draft tokens")
	}

	ticket := make(chan struct{}, 40)

	wg := sync.WaitGroup{}

	for _, snapshot := range tokensResponse {
		var token DraftToken
		err = snapshot.DataTo(&token)
		if err != nil {
			fmt.Println("Error reading snapshot into draft token: ", err)
			return err
		}
		if len(token.Roster.DST) == 0 {
			fmt.Println("This card does not have a roster card ", token.CardId)
			continue
		}
		ticket <- struct{}{} // would block if guard channel is already filled
		wg.Add(1)
		go scores.ScoreCards(&token, gameweek, &wg, ticket)
	}
	fmt.Println("Looped through all draft tokens and are waiting for them to finish")

	wg.Wait()
	fmt.Println("Finished scoring all draft tokens and returning to http function")

	return nil
}

type ScoreDraftTokensEndpoint struct {
	Scores   []Score `json:"scores"`
	GameWeek string  `json:"gameWeek"`
}

func ScoreDraftTokensEndPoint(w http.ResponseWriter, r *http.Request) {
	var reqData ScoreDraftTokensEndpoint

	err := json.NewDecoder(r.Body).Decode(&reqData)
	if err != nil {
		fmt.Println("Error decoding request body in score draft token endpoint: ", err)
		http.Error(w, fmt.Sprint("Error decoding request body in score draft token endpoint: ", err), http.StatusBadRequest)
		return
	}

	scores := Scores{
		FantasyPoints: reqData.Scores,
	}

	err = ScoreDraftTokens(reqData.GameWeek, scores)
	if err != nil {
		fmt.Println("Error scoring tokens in score draft token endpoint: ", err)
		http.Error(w, fmt.Sprint("Error scoring tokens in score draft token endpoint: ", err), http.StatusInternalServerError)
		return
	}

	data := []byte("Finished scoring draft tokens")
	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		fmt.Println("Error writing data responde: ", err)
		return
	}

	fmt.Println("Returned from ScoreDraftTokensEndpoint")
	return
}
