# Codebreaker

A terminal version of the Code Breaker board game, written in Go. Two players take turns: one makes a secret code of 5 different colors, the other tries to break it in 12 guesses or fewer. The breaker scores more for solving in fewer rows, and the first player to reach 30 points wins.

> Status: early development. v0.4.0 plays against the computer: a full match (first to 30 points, with roles swapping each round) or a Single game in either role. Playing against another person comes next.

## Play

```sh
make run
```

Type a guess as 5 color letters (`RGYBK`, `r g y b k` or `R,G,Y,B,K`). After each guess, `●` means a right color in the right spot and `△` a right color in the wrong spot. Type `?` for help or `q` to quit at any prompt.

## Documents
- [Requirements](docs/requirements.md): the game rules and the decisions behind them
- [Implementation plan](docs/implementation-plan.md): architecture, milestones, and testing

## Development

Requires Go 1.24 or later.

```sh
make run    # build and play
make test   # run the tests
make lint   # go vet + staticcheck
make help   # list all targets
```

## Learning Go with this repo
The code is heavily commented on purpose. Each package's `doc.go` says which Go concepts it shows, and idioms are explained in comments that start with `Go note:`. To list them all:

```sh
grep -rn "Go note:" internal/ cmd/
```
