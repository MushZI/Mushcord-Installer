package main

import (
	"image/color"
	"vencord/buildinfo"
)

// Personnalisation Mushcord
const Name = "Mushcord Installer"
const DataDir = "mushcord"
const AsarName = "mushcord.asar"

const ReleaseUrl = "https://api.github.com/repos/MushZI/MushZicord/releases/latest"
const ReleaseUrlFallback = "https://github.com/MushZI/MushZicord/releases/latest"
const InstallerReleaseUrl = "https://api.github.com/repos/MushZI/Mushcord-Installer/releases/latest"
const InstallerReleaseUrlFallback = "https://github.com/MushZI/Mushcord-Installer/releases/latest"

var UserAgent = "Mushcord-Installer/" + buildinfo.InstallerGitHash + " (https://github.com/MushZI/Mushcord-Installer)"

var (
	DiscordGreen  = color.RGBA{R: 0x2D, G: 0x7C, B: 0x46, A: 0xFF}
	DiscordRed    = color.RGBA{R: 0xEC, G: 0x41, B: 0x44, A: 0xFF}
	DiscordBlue   = color.RGBA{R: 0x58, G: 0x65, B: 0xF2, A: 0xFF}
	DiscordYellow = color.RGBA{R: 0xfe, G: 0xe7, B: 0x5c, A: 0xff}
)

var LinuxDiscordNames = []string{
	"Discord", "DiscordPTB", "DiscordCanary", "DiscordDevelopment",
	"discord", "discordptb", "discordcanary", "discorddevelopment",
	"discord-ptb", "discord-canary", "discord-development",
	"com.discordapp.Discord", "com.discordapp.DiscordPTB",
	"com.discordapp.DiscordCanary", "com.discordapp.DiscordDevelopment",
}
