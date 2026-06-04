SKILL_SRC := skills/lth-grv.md
SKILL_DIR := $(HOME)/.claude/skills/lth-grv

.PHONY: install install-skill install-all build test test-race test-cover clean

# Install the grv binary to ~/go/bin via go install
install:
	go install .
	@echo "Installed: $$(which grv)"

# Install the lth-grv Claude Code skill
install-skill:
	@mkdir -p $(SKILL_DIR)
	@cp $(SKILL_SRC) $(SKILL_DIR)/SKILL.md
	@echo "Installed: $(SKILL_DIR)/SKILL.md ($$(wc -l < $(SKILL_DIR)/SKILL.md) lines)"

# Install both binary and skill
install-all: install install-skill

build:
	go build -o grv .

test:
	go test ./...

test-race:
	go test -race ./...

test-cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

clean:
	rm -f grv coverage.out
