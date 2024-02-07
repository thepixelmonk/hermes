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
    "crypto/hmac"
    "crypto/sha256"
    "encoding/hex"
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
    New interface{} `json:"new"`
    Old interface{} `json:"old"`
}

type Reference struct {
    ID         interface{}    `json:"id"`
    EntityType string `json:"entity_type"`
    Name       string `json:"name"`
}

func shortcutHandler(w http.ResponseWriter, r *http.Request) { 
    var payload WebhookEvent
    var token = os.Getenv("SC_DISCORD_TOKEN")
    var secret = os.Getenv("SC_WEBHOOK_SECRET")
    discord, err := discordgo.New(token); if err != nil { log.Fatal("Invalid auth token") }
    discord.Identify.Intents = discordgo.IntentsGuildMembers
    signature := r.Header.Get("Payload-Signature")

    if r.Method != http.MethodPost {
        http.Error(w, "Method is not supported.", http.StatusMethodNotAllowed)
        return
    }

    body, err := io.ReadAll(r.Body)
    if err != nil {
        http.Error(w, "Error reading request body", http.StatusInternalServerError)
        return
    }
    r.Body = io.NopCloser(r.Body)

    if len(signature) > 0 {
        hash := hmac.New(sha256.New, []byte(secret))
        _, err = hash.Write(body)
        if err != nil {
            http.Error(w, "Error computing HMAC", http.StatusInternalServerError)
            return
        }
        computed := hex.EncodeToString(hash.Sum(nil))

        if !hmac.Equal([]byte(computed), []byte(signature)) {
            http.Error(w, "Invalid signature", http.StatusUnauthorized)
            return
        }
    }
    err = json.Unmarshal(body, &payload)  
    if err != nil {
        log.Fatal(err)
        http.Error(w, "Error parsing request body", http.StatusBadRequest)
        return
    }

    shortcutPayload(payload, discord)
    
    w.WriteHeader(http.StatusOK)
    w.Write([]byte("Received"))
}

func shortcutPayload(event WebhookEvent, discord *discordgo.Session) {
    var server = os.Getenv("SC_DISCORD_SERVER")
    var channel = os.Getenv("SC_DISCORD_CHANNEL")

    for _, action := range event.Actions {
        user := fetchUserName(action.AuthorID)
        members, err := discord.GuildMembers(server, "", 1000)

        if err == nil {
            for _, member := range members {
                if user == member.User.Username {
                    user = fmt.Sprintf("<@%s>", member.User.ID)
                }
            }
        }

        switch action.EntityType {
        case "story":
            switch action.Action {
            case "create":
                discord.ChannelMessageSend(channel, fmt.Sprintf("%s created a new story: [%s](<%s>)", user, action.Name, action.AppURL))
            case "update":
                story := fetchStoryName(action.AppURL)
                state := fetchWorkflowState(action.Changes["workflow_state_id"].New, event.References)
                switch state {
                case "Todo":
                    discord.ChannelMessageSend(channel, fmt.Sprintf("%s moved a story into Todo: [%s](<%s>)", user, story, action.AppURL))
                case "In Progress":
                    discord.ChannelMessageSend(channel, fmt.Sprintf("%s started working on: [%s](<%s>)", user, story, action.AppURL))
                case "Done":
                    discord.ChannelMessageSend(channel, fmt.Sprintf("%s completed: [%s](<%s>)", user, story, action.AppURL))
                }
            }
        case "story-comment":
            switch action.Action {
            case "create":
                story := fetchStoryName(action.AppURL)
                discord.ChannelMessageSend(channel, fmt.Sprintf("%s made a new comment on [%s](<%s>): %s", user, story, action.AppURL, action.Text))
            }
        }
    }
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

    return story.Name
}

func fetchWorkflowState(id interface{}, refs []Reference) string {
    for _, ref := range refs {
       if id == ref.ID {
            return ref.Name
        } 
    }

    return ""
}
