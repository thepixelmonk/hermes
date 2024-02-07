package main

import (
    "os"
    "io"
    "log"
    "fmt"
    "time"
    "strings"
    "net/url"
    "net/http"
    "encoding/json"
    "github.com/bwmarrin/discordgo"
)

type ShortcutMember struct {
    ID string `json:"id"`
    Name string `json:"name"`
    MentionName string `json:"mention_name"`
}

type ShortcutStory struct {
    ID   int    `json:"id"`
    Name string `json:"name"`
}

type WebhookEvent struct {
    ID         string      `json:"id"`
    ChangedAt  time.Time   `json:"changed_at"`
    PrimaryID  int         `json:"primary_id"`
    MemberID   string      `json:"member_id"`
    Version    string      `json:"version"`
    Actions    []Action    `json:"actions"`
    References []Reference `json:"references"`
}

type Action struct {
    ID         int                `json:"id"`
    AuthorID   string             `json:"author_id"`
    EntityType string             `json:"entity_type"`
    Action     string             `json:"action"`
    Name       string             `json:"name"`
    Text       string             `json:"text"`
    AppURL     string             `json:"app_url"`
    Changes    map[string]Change  `json:"changes"`
}

type Change struct {
    New   interface{}   `json:"new"`
    Old   interface{}   `json:"old"`
    Adds  []interface{} `json:"adds"`
}

type Reference struct {
    ID         int    `json:"id"`
    EntityType string `json:"entity_type"`
    Name       string `json:"name"`
}

func main() {
    var port = os.Getenv("SC_PORT")
    http.HandleFunc("/", webhookHandler)
    fmt.Println("Listening on :" + port + " ...\n")
    if err := http.ListenAndServe(":" + port, nil); err != nil { log.Fatal(err) }
}

func webhookHandler(w http.ResponseWriter, r *http.Request) { 
    var token = os.Getenv("SC_DISCORD_TOKEN")
    discord, err := discordgo.New(token); if err != nil { log.Fatal("Invalid auth token") }
    discord.Identify.Intents = discordgo.IntentsGuildMembers

    if r.Method != http.MethodPost {
        http.Error(w, "Method is not supported.", http.StatusMethodNotAllowed)
        return
    }

    body, err := io.ReadAll(r.Body)
    if err != nil {
        http.Error(w, "Error reading request body", http.StatusInternalServerError)
        return
    }
    
    var payload WebhookEvent
    err = json.Unmarshal(body, &payload)
    if err != nil {
        http.Error(w, "Error parsing request body", http.StatusBadRequest)
        return
    }
    
    handlePayload(payload, discord)
    
    w.WriteHeader(http.StatusOK)
    w.Write([]byte("Received"))
}

func handlePayload(event WebhookEvent, discord *discordgo.Session) {
    var server = os.Getenv("SC_DISCORD_SERVER")
    var channel = os.Getenv("SC_DISCORD_CHANNEL")

    for _, action := range event.Actions {
        switch action.EntityType {
        case "story-comment":
            switch action.Action {
            case "create":
                user := fetchUserName(action.AuthorID)
                story := fetchStoryName(action.AppURL)
                members, err := discord.GuildMembers(server, "", 1000)

                if err == nil {
                    for _, member := range members {
                        if user == member.User.Username {
                            user = fmt.Sprintf("<@%s>", member.User.ID)
                        }
                    }
                }

                discord.ChannelMessageSend(channel, fmt.Sprintf("%s made a new comment on [%s](<%s>): %s", user, story, action.AppURL, action.Text))
            }
        }
    }

    return
}

func fetchUserName(id string) string {
    var token = os.Getenv("SC_SHORTCUT_TOKEN")

    client := &http.Client{}
    req, err := http.NewRequest("GET", "https://api.app.shortcut.com/api/v3/member", nil); if err != nil { log.Fatal("Error creating request: ", err) }
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Shortcut-Token", token)

    resp, err := client.Do(req); if err != nil { log.Fatal("Error making request: ", err) }; defer resp.Body.Close()
    body, err := io.ReadAll(resp.Body); if err != nil { log.Fatal("Error reading response body: ", err) }

    var member ShortcutMember
    err = json.Unmarshal(body, &member); if err != nil { log.Fatal("Error parsing request body: ", err) }

    return member.MentionName
}

func fetchStoryName(uri string) string {
    var token = os.Getenv("SC_SHORTCUT_TOKEN")
    parsed, err := url.Parse(uri); if err != nil { log.Fatal(err) }
    segments := strings.Split(parsed.Path, "/")
    storyID := segments[3]

    client := &http.Client{}
    req, err := http.NewRequest("GET", "https://api.app.shortcut.com/api/v3/stories/" + storyID, nil); if err != nil { log.Fatal("Error creating request: ", err) }
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Shortcut-Token", token)

    resp, err := client.Do(req); if err != nil { log.Fatal("Error making request: ", err) }; defer resp.Body.Close()
    body, err := io.ReadAll(resp.Body); if err != nil { log.Fatal("Error reading response body: ", err) }

    var story ShortcutStory
    err = json.Unmarshal(body, &story); if err != nil { log.Fatal("Error parsing request body: ", err) }
    fmt.Printf("%+v\n", story)
    fmt.Println(storyID)

    return story.Name
}
