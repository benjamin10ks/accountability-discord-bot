package main

import "github.com/bwmarrin/discordgo"

var commands = []*discordgo.ApplicationCommand{
	{
		Name:        "register",
		Description: "Register a GitHub repository to watch",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "repo",
				Description: "Repository in format owner/repo",
				Required:    true,
			},
		},
	},
}
