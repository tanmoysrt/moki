package main

// Help pages. "moki help" shows every command and the flags of moki itself.
// "moki help <command>" and "moki <command> --help" show one command.

import (
	"fmt"
	"strings"
)

func helpCommand(_ Config, args []string) error {
	if len(args) == 0 {
		commands[0].printHelp()
		return nil
	}
	c := findCommand(args[0])
	if c == nil {
		return unknownCommand(args[0])
	}
	c.printHelp()
	return nil
}

func (c *command) printHelp() {
	if c == commands[0] {
		printMainHelp()
		return
	}
	fmt.Println()
	heading("Usage")
	printLines(c.lines)
	if c.about != "" {
		fmt.Println()
		for _, line := range strings.Split(c.about, "\n") {
			fmt.Println("  " + line)
		}
	}
	c.printSettings()
	fmt.Println()
}

func printMainHelp() {
	printLogo()
	fmt.Println("  A local viewer for UI mockups. Agents write them over MCP, you look in the browser.")
	fmt.Println()
	heading("Commands")
	var lines [][2]string
	for _, c := range commands {
		lines = append(lines, c.lines...)
	}
	printLines(lines)
	commands[0].printSettings()
	fmt.Println()
	fmt.Println("  " + dim(`Run "moki help <command>" for more about a command.`))
	fmt.Println()
}

func heading(name string) {
	fmt.Println("  " + bold(name))
}

func printLines(lines [][2]string) {
	width := 0
	for _, l := range lines {
		width = max(width, len(l[0]))
	}
	for _, l := range lines {
		fmt.Println("    " + pad(l[0], width+2) + dim(l[1]))
	}
}

// printSettings lists the flags of the command, then the env vars it reads.
func (c *command) printSettings() {
	if len(c.flags) > 0 {
		fmt.Println()
		heading("Flags")
		var rows [][3]string
		for _, name := range c.flags {
			s := find(name)
			rows = append(rows, [3]string{strings.TrimSpace("--" + s.flag + " " + s.arg), s.help, strings.TrimPrefix(s.env+shownDefault(s), ", ")})
		}
		printTable(rows)
	}
	if len(c.env) > 0 {
		fmt.Println()
		heading("Env")
		var rows [][3]string
		for _, name := range c.env {
			s := find(name)
			rows = append(rows, [3]string{s.env, s.help, strings.TrimPrefix(shownDefault(s), ", ")})
		}
		printTable(rows)
	}
}

func shownDefault(s setting) string {
	switch {
	case s.flag == "url":
		return ", from host and port"
	case s.def != "":
		return ", " + s.def
	}
	return ""
}

func printTable(rows [][3]string) {
	w0, w1 := 0, 0
	for _, r := range rows {
		w0, w1 = max(w0, len(r[0])), max(w1, len(r[1]))
	}
	for _, r := range rows {
		fmt.Println(strings.TrimRight("    "+pad(r[0], w0+2)+pad(r[1], w1+2)+dim(r[2]), " "))
	}
}

func find(flag string) setting {
	for _, s := range settings {
		if s.flag == flag {
			return s
		}
	}
	panic("no setting " + flag)
}
