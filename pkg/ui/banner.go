package ui

import (
	"fmt"
	"os"
)

var bannerCLAB = []string{
	" ⣴⡾⠛⠛⠖ ⢸⣿      ⣾⣿⡀  ⢸⣿⠛⠛⣷⡄",
	"⢸⣿     ⢸⣿     ⣸⡏⢹⣧  ⢸⣿⣀⣀⣾⠇",
	"⠘⣿⣄  ⡀ ⢸⣿    ⢠⣿⠷⠶⢿⡆ ⢸⣿⠉⠉⣷⡆",
	" ⠈⠙⠛⠛⠉ ⠘⠛⠛⠛⠛  ⠚⠃ ⠘⠛ ⠘⠛⠛⠛⠋ ",
}

var bannerErnetes = []string{
	"                               ⢸⡇               ",
	" ⢠⣶⠟⠛⢷⣦ ⢸⣿⠛⠛⣷⡄ ⢸⣿⠛⠛⣷⣦ ⢠⣶⠟⠛⢷⣦ ⠘⠛⣿⡟⠛ ⢠⣶⠟⠛⢷⣦ ⣴⡟⠛⠛⢻⣦",
	" ⢸⣿⣤⣤⠾⠃ ⢸⣿     ⢸⣿  ⢸⣿ ⢸⣿⣤⣤⠾⠃   ⢸⡇  ⢸⣿⣤⣤⠾⠃ ⣈⡛⠛⠛⣿⡆",
	"  ⠈⠙⠛⠛⠉ ⠘⠛     ⠘⠛  ⠘⠛  ⠈⠙⠛⠛⠉   ⠘⠛   ⠈⠙⠛⠛⠉ ⠈⠛⠛⠛⠛⠁",
}

// BrailleBanner is the containerlab-styled braille logo for CLABernetes.
const BrailleBanner = "" +
	" ⣴⡾⠛⠛⠖ ⢸⣿      ⣾⣿⡀  ⢸⣿⠛⠛⣷⡄                               ⢸⡇               \n" +
	"⢸⣿     ⢸⣿     ⣸⡏⢹⣧  ⢸⣿⣀⣀⣾⠇ ⢠⣶⠟⠛⢷⣦ ⢸⣿⠛⠛⣷⡄ ⢸⣿⠛⠛⣷⣦ ⢠⣶⠟⠛⢷⣦ ⠘⠛⣿⡟⠛ ⢠⣶⠟⠛⢷⣦ ⣴⡟⠛⠛⢻⣦\n" +
	"⠘⣿⣄  ⡀ ⢸⣿    ⢠⣿⠷⠶⢿⡆ ⢸⣿⠉⠉⣷⡆ ⢸⣿⣤⣤⠾⠃ ⢸⣿     ⢸⣿  ⢸⣿ ⢸⣿⣤⣤⠾⠃   ⢸⡇  ⢸⣿⣤⣤⠾⠃ ⣈⡛⠛⠛⣿⡆\n" +
	" ⠈⠙⠛⠛⠉ ⠘⠛⠛⠛⠛  ⠚⠃ ⠘⠛ ⠘⠛⠛⠛⠋   ⠈⠙⠛⠛⠉ ⠘⠛     ⠘⠛  ⠘⠛  ⠈⠙⠛⠛⠉   ⠘⠛   ⠈⠙⠛⠛⠉ ⠈⠛⠛⠛⠛⠁"

// PrintBanner outputs the CLABernetes logo banner with styling similar to containerlab.
func PrintBanner() {
	if os.Getenv("C9S_NO_BANNER") != "" {
		return
	}
	for i := 0; i < len(bannerCLAB); i++ {
		fmt.Printf("%s%s%s%s%s%s\n", ColorWhite, bannerCLAB[i], ColorReset, ColorClabBlue, bannerErnetes[i], ColorReset)
	}
}
