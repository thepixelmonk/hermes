package main

import (
    "io"
    "os"
    "log"
    "fmt"
    "net/http"
    "encoding/json"
    "github.com/bwmarrin/discordgo"
)

type GithubPayload struct {
    Ref        string     `json:"ref"`
    Repo GithubRepo `json:"repository"`
    Commits    []Commit   `json:"commits"`
    HeadCommit Commit     `json:"head_commit"`
}

type GithubRepo struct {
    ID               int64  `json:"id"`
    Name             string `json:"name"`
    FullName         string `json:"full_name"`
    Owner            User   `json:"owner"`
    HtmlUrl          string `json:"html_url"`
    Url              string `json:"url"`
}

type User struct {
    Name            string `json:"name"`
    Email           string `json:"email"`
    ID              int64  `json:"id"`
    AvatarUrl       string `json:"avatar_url"`
    Url             string `json:"url"`
}

type Commit struct {
    ID        string    `json:"id"`
    Message   string    `json:"message"`
    Timestamp string    `json:"timestamp"`
    Url       string    `json:"url"`
    Author    Committer `json:"author"`
    Committer Committer `json:"committer"`
}

type Committer struct {
    Name     string `json:"name"`
    Email    string `json:"email"`
    Username string `json:"username"`
}

func githubHandler(w http.ResponseWriter, r *http.Request) { 
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
    
    var payload GithubPayload
    err = json.Unmarshal(body, &payload)
    if err != nil {
        http.Error(w, "Error parsing request body", http.StatusBadRequest)
        return
    }
    
    githubPayload(payload, discord, r)
    
    w.WriteHeader(http.StatusOK)
    w.Write([]byte("Received"))
}

func githubPayload(payload GithubPayload, discord *discordgo.Session, r *http.Request) {
    var channel = os.Getenv("SC_DISCORD_CHANNEL")

    event := r.Header.Get("X-GitHub-Event")
    switch event {
    case "push":
        num := len(payload.Commits)
        user := payload.HeadCommit.Author.Username
        if num == 1 {
            discord.ChannelMessageSend(channel, fmt.Sprintf("%s pushed a new [commit](<%s>) to [%s](<%s>): %s", user, payload.HeadCommit.Url, payload.Repo.Name, payload.Repo.Url, payload.HeadCommit.Message))
        } else {
            message := fmt.Sprintf("%s pushed %d new commits to [%s](<%s>):", user, num, payload.Repo.Name, payload.Repo.Url)
            for _, commit := range payload.Commits {
                message += fmt.Sprintf("\n- [%s](<%s>): %s", commit.ID[:7], commit.Url, commit.Message)
            }
            discord.ChannelMessageSend(channel, message)
        }
    }
}

