package cloudfunctions

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/CJPotter10/sbs-cloud-functions-api/utils"
)

type LeagueUser struct {
	OwnerId string `json:"ownerId"`
	TokenId string `json:"tokenId"`
}

type League struct {
	LeagueId     string       `json:"leagueId" firestore:"LeagueId"`
	DisplayName  string       `json:"displayName" firestore:"DisplayName"`
	CurrentUsers []LeagueUser `json:"currentUsers" firestore:"CurrentUsers"`
	NumPlayers   int          `json:"numPlayers" firestore:"NumPlayers"`
	MaxPlayers   int          `json:"maxPlayers" firestore:"MaxPlayers"`
	StartDate    time.Time    `json:"startDate" firestore:"StartDate"`
	EndDate      time.Time    `json:"endDate" firestore:"EndDate"`
	DraftType    string       `json:"draftType" firestore:"DraftType"`
	Level        string       `json:"level" firestore:"Level"`
	IsLocked     bool         `json:"isLocked" firestore:"IsLocked"`
}

type PlayerInfo struct {
	// unique player Id will probably just be the team and position such as BUFQB
	PlayerId string `json:"playerId"`
	// display name for front end
	DisplayName string `json:"displayName"`
	// team of the player
	Team string `json:"team"`
	// position of player
	Position string `json:"position"`
	// address of the user who drafted this player
	OwnerAddress string `json:"ownerAddress"`
	// number pick that this player was selected.... will default to nil in the database
	PickNum int `json:"pickNum"`
	// the round which this player was drafted in
	Round int `json:"round"`
}

type DraftSummary struct {
	Summary []PlayerInfo `json:"summary"`
}

type DraftPositionTracker struct {
	Players map[string][]int `json:"players"`
}

type PickInfo struct {
	PlayerId string
	PickNum  int
}

// func createLeagueObject(data interface{}) League {
// 	dataMap := data.(map[string]interfa)
// 	return League{
// 		LeagueId: data["LeagueId"].(string),
// 	}
// }

func CalculateADP(w http.ResponseWriter, r *http.Request) {
	res, err := utils.Db.Client.Collection("drafts").Documents(context.Background()).GetAll()
	if err != nil {
		fmt.Println("error reading all league documents: ", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	wgMain := sync.WaitGroup{}

	pickNumChan := make(chan PickInfo)
	stopChannel := make(chan string)

	wgMain.Add(1)
	go ListenForPickNumbers(pickNumChan, stopChannel, &wgMain)

	wg := sync.WaitGroup{}

	ticket := make(chan struct{}, 40)

	for i := 0; i < len(res); i++ {
		var league League
		err = res[i].DataTo(&league)
		if err != nil {
			fmt.Println("Error reading league data into object: ", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		fmt.Println("League Id: ", league.LeagueId)
		if !league.IsLocked {
			fmt.Printf("This league: %s is not locked so we are skipping it\r", league.LeagueId)
			http.Error(w, fmt.Sprintf("This league: %s is not locked so we are skipping it\r", league.LeagueId), http.StatusInternalServerError)
			continue
		}

		ticket <- struct{}{} // would block if guard channel is already filled
		wg.Add(1)
		go func() {
			defer func() {
				wg.Done()
				<-ticket
			}()
			data, err := utils.Db.Client.Collection(fmt.Sprintf("drafts/%s/state", league.LeagueId)).Doc("summary").Get(context.Background())
			if err != nil {
				fmt.Println("Error reading draft summary in adp calculator: ", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			var summary DraftSummary
			err = data.DataTo(&summary)
			if err != nil {
				fmt.Println("Error reading data into draft summary: ", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			for i := 0; i < len(summary.Summary); i++ {
				pick := summary.Summary[i]
				pickNumChan <- PickInfo{PlayerId: pick.PlayerId, PickNum: pick.PickNum}
			}
			fmt.Println("Finihed looping through for ", league.LeagueId)
		}()

	}
	fmt.Println("Out of for loop in main function and are waiting for all league processing to finish")

	wg.Wait()

	fmt.Println("done going through leagues and are sending complete signal to go routine to calculate adp")
	stopChannel <- "complete"
	fmt.Println("Completed the for loop and am writing to stop channel")

	wgMain.Wait()

	returnData, err := json.Marshal([]byte("Completed Updated ADP"))
	if err != nil {
		fmt.Println("Error marshalling stats object for return data: ", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(returnData)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		fmt.Println("Error writing data responde: ", err)
		return
	}
	fmt.Println("All go routines have finished and adp has been calculated. Everything is done")
	return
}

type StatsObject struct {
	PlayerId        string   `json:"playerId"`
	AverageScore    float64  `json:"averageScore"`
	HighestScore    float64  `json:"highestScore"`
	Top5Finishes    int64    `json:"top5Finishes"`
	ByeWeek         string   `json:"byeWeek"`
	ADP             float64  `json:"adp"`
	PlayersFromTeam []string `json:"playersFromTeam"`
}

type StatsMap struct {
	Players map[string]StatsObject `json:"players"`
}

func ListenForPickNumbers(pick chan PickInfo, stopChan chan string, wg *sync.WaitGroup) {
	defer wg.Done()
	tracker := DraftPositionTracker{
		Players: make(map[string][]int, 0),
	}

	fmt.Println("Entering loop to listen for pick nums")
Loop:
	for {
		select {
		case data := <-pick:
			fmt.Printf("data recieved: %s, %d", data.PlayerId, data.PickNum)
			tracker.Players[data.PlayerId] = append(tracker.Players[data.PlayerId], data.PickNum)
		case mes := <-stopChan:
			if mes == "complete" {
				fmt.Println("Recieved complete message")
				break Loop
			}
		}
	}

	fmt.Println("Out of loop listening for pick nums")

	stats := StatsMap{
		Players: make(map[string]StatsObject),
	}
	data, err := utils.Db.Client.Collection("playerStats2023").Doc("playerMap").Get(context.Background())
	if err != nil {
		fmt.Println("Error reading statsMap: ", err)
		return
	}

	err = data.DataTo(&stats)
	if err != nil {
		fmt.Println("Error reading data into statsMap: ", err)
		return
	}

	newStatsMap := StatsMap{
		Players: make(map[string]StatsObject),
	}

	for playerId, obj := range tracker.Players {
		sum := 0
		for i := 0; i < len(obj); i++ {
			sum = sum + obj[i]
		}

		avg := float64(sum / len(obj))
		statsObj := stats.Players[playerId]
		statsObj.ADP = avg
		stats.Players[playerId] = statsObj

		// newStatsMap.Players[playerId] = StatsObject{
		// 	PlayerId:        stats.Players[playerId].PlayerId,
		// 	Top5Finishes:    stats.Players[playerId].Top5Finishes,
		// 	AverageScore:    stats.Players[playerId].AverageScore,
		// 	HighestScore:    stats.Players[playerId].HighestScore,
		// 	ByeWeek:         stats.Players[playerId].ByeWeek,
		// 	ADP:             avg,
		// 	PlayersFromTeam: stats.Players[playerId].PlayersFromTeam,
		// }
	}

	for key, value := range stats.Players {
		fmt.Printf("key: %s, value: %v\r", key, value)
		if key == "" {
			fmt.Println("Found the weird ass thing in the object and skipping it")
			continue
		}
		newStatsMap.Players[key] = stats.Players[key]
	}

	// _, err = utils.Db.Client.Collection("playerStats2023").Doc("newPlayerMap").Update(context.Background(), []db.Update{
	// 	{
	// 		Path:  "players",
	// 		Value: newStatsMap.Players,
	// 	},
	// })

	err = utils.Db.CreateOrUpdateDocument("playerStats2023", "newPlayerMap", newStatsMap)
	if err != nil {
		fmt.Println("Error updating playerStats2023/playerMap: ", err)
		return
	}

	fmt.Println("Updated ADP")

}
