package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/biggs-100/biggz-ai/internal/bigmem"
)

// bigmemRun handles the "biggz bigmem" subcommand.
func bigmemRun() int {
	// Fast-path for `biggz bigmem sync --help` without opening DB (test isolation and disk-error resilience)
	if len(os.Args) >= 4 && os.Args[2] == "sync" {
		for _, a := range os.Args[3:] {
			if a == "--help" || a == "-h" {
				fmt.Fprintln(os.Stderr, "Usage: biggz bigmem sync [--import] [--status] [--project P] [--all] [--from-engram] [--engram-dir PATH]")
				fmt.Fprintln(os.Stderr, "  (no flags)      Export observations to .bigmem/ in project")
				fmt.Fprintln(os.Stderr, "  --import        Import observations from .bigmem/")
				fmt.Fprintln(os.Stderr, "  --status        Show .bigmem/ status")
				fmt.Fprintln(os.Stderr, "  --project NAME  Filter export to a project")
				fmt.Fprintln(os.Stderr, "  --all           Export ALL projects (ignore cwd filter)")
				fmt.Fprintln(os.Stderr, "  --from-engram       Import from .engram/ instead of .bigmem/")
				fmt.Fprintln(os.Stderr, "  --engram-dir PATH   Engram directory (default .engram)")
				return 1
			}
		}
	}
	store, err := bigmem.Open("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: open bigmem: %v\n", err)
		return 1
	}
	defer store.Close()

	args := os.Args[2:]
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprintln(os.Stderr, "Usage: biggz bigmem <command> [args...]")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Commands:")
		fmt.Fprintln(os.Stderr, "  save <title> <msg> [--type T] [--project P] [--scope S]")
		fmt.Fprintln(os.Stderr, "  search <query> [--type T] [--project P] [--scope S] [--limit N]")
		fmt.Fprintln(os.Stderr, "  get <id>                        Get an observation by ID")
		fmt.Fprintln(os.Stderr, "  delete <id>                     Delete an observation")
		fmt.Fprintln(os.Stderr, "  update <id> [flags]             Update an observation")
		fmt.Fprintln(os.Stderr, "    --title <t>   New title")
		fmt.Fprintln(os.Stderr, "    --type <t>    New type")
		fmt.Fprintln(os.Stderr, "    --content <c> New content")
		fmt.Fprintln(os.Stderr, "    --topic-key <k> New topic key")
		fmt.Fprintln(os.Stderr, "    --scope <s>   New scope")
		fmt.Fprintln(os.Stderr, "  timeline <id>                   Chronological context")
		fmt.Fprintln(os.Stderr, "    --before <n>  Entries before (default: 5)")
		fmt.Fprintln(os.Stderr, "    --after <n>   Entries after (default: 5)")
		fmt.Fprintln(os.Stderr, "  context [project]               Recent session context")
		fmt.Fprintln(os.Stderr, "  stats                           Show memory statistics")
		fmt.Fprintln(os.Stderr, "  doctor                          Run store diagnostics")
		fmt.Fprintln(os.Stderr, "  export [file]                   Export all memories to JSON")
		fmt.Fprintln(os.Stderr, "  import <file>                   Import memories from JSON")
		fmt.Fprintln(os.Stderr, "  projects list                   List all projects")
		fmt.Fprintln(os.Stderr, "  compare <id-a> <id-b>           Compare two observations")
		fmt.Fprintln(os.Stderr, "  conflicts list                  List pending memory conflicts")
		fmt.Fprintln(os.Stderr, "  conflicts show <id>             Show a relation with observations")
		fmt.Fprintln(os.Stderr, "  conflicts judge <id> <verdict>   Judge a conflict")
		fmt.Fprintln(os.Stderr, "    verdicts: related, compatible, scoped, conflicts_with, supersedes, not_conflict")
		fmt.Fprintln(os.Stderr, "  conflicts scan [--project P]     Scan for new conflicts")
		fmt.Fprintln(os.Stderr, "  graph [--project P] [--format dot|ascii|json] [--limit N] [--scope project|all]")
		fmt.Fprintln(os.Stderr, "                                Render topic_key hierarchy and relations")
		fmt.Fprintln(os.Stderr, "  version                         Show bigmem version")
		fmt.Fprintln(os.Stderr, "  help                            Show this help")
		return 1
	}

	switch args[0] {
	case "save":
		if len(args) < 3 {
			fmt.Fprintln(os.Stderr, "Usage: biggz bigmem save <title> <msg> [--type T] [--project P] [--scope S]")
			return 1
		}
		obs := &bigmem.Observation{Title: args[1], Content: args[2], Type: "manual"}
		for i := 3; i < len(args)-1; i++ {
			switch args[i] {
			case "--type":
				obs.Type = args[i+1]
				i++
			case "--project":
				obs.Project = args[i+1]
				i++
			case "--scope":
				obs.Scope = args[i+1]
				i++
			}
		}
		if err := store.Save(obs); err != nil {
			fmt.Fprintf(os.Stderr, "error: save: %v\n", err)
			return 1
		}
		fmt.Printf("Saved: %s\n", obs.ID)

	case "search":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: biggz bigmem search <query> [--type T] [--project P] [--scope S] [--limit N]")
			return 1
		}
		query := args[1]
		opts := bigmem.SearchOptions{Limit: 20}
		for i := 2; i < len(args)-1; i++ {
			switch args[i] {
			case "--type":
				opts.Type = args[i+1]
				i++
			case "--project":
				opts.Project = args[i+1]
				i++
			case "--scope":
				opts.Scope = args[i+1]
				i++
			case "--limit":
				if n, err := strconv.Atoi(args[i+1]); err == nil {
					opts.Limit = n
				}
				i++
			}
		}
		results, err := store.Search(query, opts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: search: %v\n", err)
			return 1
		}
		if len(results) == 0 {
			fmt.Println("No results.")
			return 0
		}
		for _, r := range results {
			ago := time.Since(r.CreatedAt).Round(time.Hour)
			fmt.Printf("  %s [%s] %s (%s)\n", r.ID[:min(20, len(r.ID))], r.Type, r.Title, ago)
		}

	case "get":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: biggz bigmem get <id>")
			return 1
		}
		obs, err := store.Get(args[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: get: %v\n", err)
			return 1
		}
		fmt.Printf("ID:        %s\n", obs.ID)
		fmt.Printf("Title:     %s\n", obs.Title)
		fmt.Printf("Type:      %s\n", obs.Type)
		fmt.Printf("TopicKey:  %s\n", obs.TopicKey)
		fmt.Printf("Project:   %s\n", obs.Project)
		fmt.Printf("Scope:     %s\n", obs.Scope)
		fmt.Printf("Created:   %s\n", obs.CreatedAt.Format(time.RFC3339))
		fmt.Printf("Updated:   %s\n", obs.UpdatedAt.Format(time.RFC3339))
		fmt.Printf("Content:   %s\n", obs.Content)

	case "delete":
		if len(args) < 2 || args[1] == "--help" || args[1] == "-h" {
			fmt.Fprintln(os.Stderr, "Usage: biggz bigmem delete <id> [--hard]")
			fmt.Fprintln(os.Stderr, "       biggz bigmem delete session <id>")
			fmt.Fprintln(os.Stderr, "       biggz bigmem delete prompt <id>")
			fmt.Fprintln(os.Stderr, "       biggz bigmem delete project <name> [--hard]")
			return 1
		}
		hard := false
		for _, a := range args {
			if a == "--hard" {
				hard = true
			}
		}
		if args[1] == "session" {
			if len(args) < 3 {
				fmt.Fprintln(os.Stderr, "Usage: biggz bigmem delete session <id>")
				return 1
			}
			if err := store.DeleteSession(args[2]); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				return 1
			}
			fmt.Printf("Session deleted: %s\n", args[2])
		} else if args[1] == "prompt" {
			if len(args) < 3 {
				fmt.Fprintln(os.Stderr, "Usage: biggz bigmem delete prompt <id>")
				return 1
			}
			id, err := strconv.ParseInt(args[2], 10, 64)
			if err != nil {
				fmt.Fprintln(os.Stderr, "invalid prompt id")
				return 1
			}
			if err := store.DeletePrompt(id); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				return 1
			}
			fmt.Printf("Prompt deleted: %s\n", args[2])
		} else if args[1] == "project" {
			if len(args) < 3 {
				fmt.Fprintln(os.Stderr, "Usage: biggz bigmem delete project <name> [--hard]")
				return 1
			}
			r, err := store.DeleteProject(args[2], hard)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				return 1
			}
			fmt.Printf("Project %q deleted: %d observations, %d prompts, %d sessions\n",
				args[2], r.ObservationsDeleted, r.PromptsDeleted, r.SessionsDeleted)
		} else {
			if err := store.DeleteObservation(args[1], hard); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				return 1
			}
			fmt.Printf("Deleted: %s\n", args[1])
		}

	case "update":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: biggz bigmem update <id> --title <t> --type <t> --content <c> --topic-key <k> --scope <s>")
			return 1
		}
		id := args[1]
		updates := map[string]any{}
		for i := 2; i < len(args)-1; i++ {
			switch args[i] {
			case "--title":
				updates["title"] = args[i+1]
				i++
			case "--type":
				updates["type"] = args[i+1]
				i++
			case "--content":
				updates["content"] = args[i+1]
				i++
			case "--topic-key":
				updates["topic_key"] = args[i+1]
				i++
			case "--scope":
				updates["scope"] = args[i+1]
				i++
			}
		}
		if len(updates) == 0 {
			fmt.Fprintln(os.Stderr, "no fields to update")
			return 1
		}
		obs, err := store.Update(id, updates)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: update: %v\n", err)
			return 1
		}
		fmt.Printf("Updated: %s\n", obs.ID)

	case "timeline":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: biggz bigmem timeline <id> [--before N] [--after N]")
			return 1
		}
		opts := bigmem.TimelineOptions{FocusID: args[1]}
		for i := 2; i < len(args); i++ {
			switch args[i] {
			case "--before":
				if i+1 < len(args) {
					opts.Before, _ = strconv.Atoi(args[i+1])
					i++
				}
			case "--after":
				if i+1 < len(args) {
					opts.After, _ = strconv.Atoi(args[i+1])
					i++
				}
			}
		}
		entries, err := store.Timeline(opts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: timeline: %v\n", err)
			return 1
		}
		for _, e := range entries {
			marker := "  "
			if e.IsFocus {
				marker = "=>"
			}
			if e.IsBefore {
				marker = "<="
			}
			fmt.Printf("%s %s [%s] %s (%s)\n", marker, e.ID[:min(20, len(e.ID))], e.Type, e.Title, e.CreatedAt.Format(time.RFC3339))
		}

	case "context":
		project := ""
		if len(args) > 1 {
			project = args[1]
		}
		sessions, err := store.SessionContext(10)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: context: %v\n", err)
			return 1
		}
		if len(sessions) == 0 {
			fmt.Println("No session history.")
			return 0
		}
		for _, s := range sessions {
			if project != "" && s.Project != project {
				continue
			}
			line := fmt.Sprintf("  %s — %s", s.ID, s.StartTime.Format("2006-01-02 15:04"))
			if !s.EndTime.IsZero() {
				line += fmt.Sprintf(" → %s", s.EndTime.Format("15:04"))
			}
			if s.Summary != "" {
				line += fmt.Sprintf(" — %s", truncateStr(s.Summary, 120))
			}
			fmt.Println(line)
		}

	case "stats":
		stats, err := store.Stats()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: stats: %v\n", err)
			return 1
		}
		fmt.Printf("Total observations: %d\n", stats.TotalObservations)
		fmt.Printf("By type:\n")
		for t, c := range stats.ByType {
			fmt.Printf("  %s: %d\n", t, c)
		}
		fmt.Printf("Total sessions: %d\n", stats.TotalSessions)
		fmt.Printf("Storage: %s\n", stats.StoragePath)

	case "doctor":
		useJSON := false
		doFix := false
		for i := 1; i < len(args); i++ {
			switch args[i] {
			case "--json":
				useJSON = true
			case "--fix":
				doFix = true
			}
		}
		if doFix {
			if err := store.DoctorFix(); err != nil {
				fmt.Fprintf(os.Stderr, "error: doctor --fix: %v\n", err)
				return 1
			}
			if !useJSON {
				fmt.Fprintln(os.Stderr, "Doctor fix applied: WAL checkpoint, VACUUM, FTS rebuild, schema migration.")
			}
		}
		r, err := store.Doctor()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		if useJSON {
			data, _ := json.MarshalIndent(r, "", "  ")
			fmt.Println(string(data))
			return 0
		}
		fmt.Printf("Store exists: %v\n", r.StoreExists)
		fmt.Printf("Observations: %d\n", r.Observations)
		fmt.Printf("Corrupt: %v\n", r.Corrupt)
		fmt.Printf("Storage: %s\n", r.StoragePath)

	case "export":
		filePath := "bigmem-export.json"
		if len(args) > 1 {
			filePath = args[1]
		}
		all, err := store.Search("", bigmem.SearchOptions{Limit: 100000})
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: export: %v\n", err)
			return 1
		}
		data, err := json.MarshalIndent(all, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: marshal: %v\n", err)
			return 1
		}
		if err := os.WriteFile(filePath, data, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "error: write %s: %v\n", filePath, err)
			return 1
		}
		fmt.Printf("Exported %d observations to %s\n", len(all), filePath)

	case "import":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: biggz bigmem import <file>")
			return 1
		}
		data, err := os.ReadFile(args[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: read %s: %v\n", args[1], err)
			return 1
		}
		var obs []*bigmem.Observation
		if err := json.Unmarshal(data, &obs); err != nil {
			fmt.Fprintf(os.Stderr, "error: parse: %v\n", err)
			return 1
		}
		imported := 0
		for _, o := range obs {
			if err := store.Save(o); err != nil {
				fmt.Fprintf(os.Stderr, "warn: save %s: %v\n", o.ID, err)
				continue
			}
			imported++
		}
		fmt.Printf("Imported %d/%d observations\n", imported, len(obs))

	case "projects":
		if len(args) < 2 || args[1] == "--help" || args[1] == "-h" {
			fmt.Fprintln(os.Stderr, "Usage: biggz bigmem projects list")
			fmt.Fprintln(os.Stderr, "       biggz bigmem projects consolidate [--all] [--dry-run]")
			return 1
		}
		if args[1] == "list" {
			rows, err := store.ListProjects()
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				return 1
			}
			if len(rows) == 0 {
				fmt.Println("No projects.")
				return 0
			}
			for _, p := range rows {
				fmt.Printf("  %s — %d observations, %d sessions\n", p.Name, p.Observations, p.Sessions)
			}
		} else if args[1] == "consolidate" {
			all := false
			dryRun := false
			for i := 2; i < len(args); i++ {
				switch args[i] {
				case "--all":
					all = true
				case "--dry-run":
					dryRun = true
				}
			}
			r, err := store.ConsolidateProjects(all, dryRun)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				return 1
			}
			if dryRun {
				if len(r.Groups) == 0 {
					fmt.Println("No similar project groups found.")
					return 0
				}
				for _, g := range r.Groups {
					fmt.Printf("Group: %s\n", g.Canonical)
					for _, p := range g.Projects[1:] {
						fmt.Printf("  → %s\n", p)
					}
				}
			} else {
				fmt.Printf("Consolidated %d projects\n", r.Merged)
			}
		} else {
			fmt.Fprintf(os.Stderr, "unknown: projects %s\n", args[1])
			return 1
		}

	case "compare":
		if len(args) < 3 {
			fmt.Fprintln(os.Stderr, "Usage: biggz bigmem compare <id-a> <id-b>")
			return 1
		}
		r, err := store.Compare(args[1], args[2])
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: compare: %v\n", err)
			return 1
		}
		fmt.Printf("A: %s — %s [%s]\n", r.A.ID[:min(20, len(r.A.ID))], r.A.Title, r.A.Type)
		fmt.Printf("B: %s — %s [%s]\n", r.B.ID[:min(20, len(r.B.ID))], r.B.Title, r.B.Type)
		fmt.Printf("Same topic: %v\n", r.SameTopic)
		fmt.Printf("Same project: %v\n", r.SameProject)
		fmt.Printf("Time diff: %s\n", r.TimeDiff)

	case "conflicts":
		if len(args) < 2 || args[1] == "--help" || args[1] == "-h" {
			fmt.Fprintln(os.Stderr, "Usage: biggz bigmem conflicts <list|show|judge|scan> [...]")
			return 1
		}
		switch args[1] {
		case "list":
			status := ""
			if len(args) > 2 {
				status = args[2]
			}
			rels, err := store.ListRelations(status)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				return 1
			}
			if len(rels) == 0 {
				fmt.Println("No relations found.")
				return 0
			}
			for _, r := range rels {
				fmt.Printf("  %s [%s] %s → %s (%s)\n",
					r.ID[:min(24, len(r.ID))], r.JudgmentStatus, r.SourceID[:min(16, len(r.SourceID))],
					r.TargetID[:min(16, len(r.TargetID))], r.Relation)
			}
		case "show":
			if len(args) < 3 {
				fmt.Fprintln(os.Stderr, "Usage: biggz bigmem conflicts show <id>")
				return 1
			}
			rel, err := store.GetRelation(args[2])
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				return 1
			}
			src, _ := store.Get(rel.SourceID)
			tgt, _ := store.Get(rel.TargetID)
			fmt.Printf("Relation: %s\n", rel.ID)
			fmt.Printf("Status:   %s\n", rel.JudgmentStatus)
			fmt.Printf("Verdict:  %s\n", rel.Relation)
			fmt.Printf("Source:\n")
			if src != nil {
				fmt.Printf("  %s — %s [%s]\n", src.ID[:min(24, len(src.ID))], src.Title, src.Type)
			}
			fmt.Printf("Target:\n")
			if tgt != nil {
				fmt.Printf("  %s — %s [%s]\n", tgt.ID[:min(24, len(tgt.ID))], tgt.Title, tgt.Type)
			}
			if rel.Reason != "" {
				fmt.Printf("Reason: %s\n", rel.Reason)
			}
			if rel.Evidence != "" {
				fmt.Printf("Evidence: %s\n", rel.Evidence)
			}
			if rel.Confidence > 0 {
				fmt.Printf("Confidence: %.2f\n", rel.Confidence)
			}
			fmt.Printf("Created: %s\n", rel.CreatedAt.Format(time.RFC3339))

		case "judge":
			if len(args) < 4 {
				fmt.Fprintln(os.Stderr, "Usage: biggz bigmem conflicts judge <id> <verdict>")
				fmt.Fprintln(os.Stderr, "  verdicts: related, compatible, scoped, conflicts_with, supersedes, not_conflict")
				return 1
			}
			reason := ""
			if len(args) > 4 {
				reason = strings.Join(args[4:], " ")
			}
			if err := store.JudgeRelation(args[2], args[3], reason, "", 1.0); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				return 1
			}
			fmt.Printf("Judged %s as %s\n", args[2][:min(24, len(args[2]))], args[3])

		case "scan":
			project := ""
			dryRun := false
			apply := false
			for i := 2; i < len(args); i++ {
				switch args[i] {
				case "--project":
					if i+1 < len(args) {
						project = args[i+1]
						i++
					}
				case "--dry-run":
					dryRun = true
				case "--apply":
					apply = true
				}
			}
			if apply {
				n, err := store.ScanConflicts(project, false)
				if err != nil {
					fmt.Fprintf(os.Stderr, "error: %v\n", err)
					return 1
				}
				fmt.Printf("Created %d pending conflict relations\n", n)
			} else {
				n, err := store.ScanConflicts(project, true)
				if err != nil {
					fmt.Fprintf(os.Stderr, "error: %v\n", err)
					return 1
				}
				if dryRun {
					fmt.Printf("Dry-run: %d observations would create new relations\n", n)
				} else {
					fmt.Printf("Would create relations for %d observations (use --apply to commit)\n", n)
				}
			}

		case "stats":
			project := ""
			for i := 2; i < len(args); i++ {
				if args[i] == "--project" && i+1 < len(args) {
					project = args[i+1]
					i++
				}
			}
			cs, err := store.ConflictsStats(project)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				return 1
			}
			fmt.Printf("Total relations: %d\n", cs.TotalRelations)
			fmt.Printf("Pending:         %d\n", cs.Pending)
			fmt.Printf("Judged:          %d\n", cs.Judged)
			if len(cs.ByVerdict) > 0 {
				fmt.Println("By verdict:")
				for v, c := range cs.ByVerdict {
					fmt.Printf("  %s: %d\n", v, c)
				}
			}

		case "deferred":
			status := ""
			limit := 20
			for i := 2; i < len(args); i++ {
				switch args[i] {
				case "--status":
					if i+1 < len(args) {
						status = args[i+1]
						i++
					}
				case "--limit":
					if i+1 < len(args) {
						limit, _ = strconv.Atoi(args[i+1])
						i++
					}
				}
			}
			rels, err := store.ConflictsDeferred(status, limit)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				return 1
			}
			if len(rels) == 0 {
				fmt.Println("No deferred relations.")
				return 0
			}
			for _, r := range rels {
				fmt.Printf("  %s %s → %s (%s)\n", r.ID[:min(24, len(r.ID))],
					r.SourceID[:min(16, len(r.SourceID))], r.TargetID[:min(16, len(r.TargetID))], r.Relation)
			}

		default:
			fmt.Fprintf(os.Stderr, "unknown: conflicts %s\n", args[1])
			return 1
		}

	case "sync":
		doImport := false
		doStatus := false
		doAll := false
		fromEngram := false
		engramDir := ""
		project := ""
		hasProject := false
		for i := 1; i < len(args); i++ {
			switch args[i] {
			case "--help", "-h":
				fmt.Fprintln(os.Stderr, "Usage: biggz bigmem sync [--import] [--status] [--project P] [--all] [--from-engram] [--engram-dir PATH]")
				fmt.Fprintln(os.Stderr, "  (no flags)      Export observations to .bigmem/ in project")
				fmt.Fprintln(os.Stderr, "  --import        Import observations from .bigmem/")
				fmt.Fprintln(os.Stderr, "  --status        Show .bigmem/ status")
				fmt.Fprintln(os.Stderr, "  --project NAME  Filter export to a project")
				fmt.Fprintln(os.Stderr, "  --all           Export ALL projects (ignore cwd filter)")
				fmt.Fprintln(os.Stderr, "  --from-engram       Import from .engram/ instead of .bigmem/")
				fmt.Fprintln(os.Stderr, "  --engram-dir PATH   Engram directory (default .engram)")
				return 1
			case "import":
				doImport = true
			case "--import":
				doImport = true
			case "--status":
				doStatus = true
			case "--all":
				doAll = true
			case "--from-engram":
				fromEngram = true
			case "--engram-dir":
				if i+1 < len(args) {
					engramDir = args[i+1]
					i++
				}
			case "--project":
				if i+1 < len(args) {
					project = args[i+1]
					hasProject = true
					i++
				}
			default:
				if strings.HasPrefix(args[i], "--engram-dir=") {
					engramDir = strings.TrimPrefix(args[i], "--engram-dir=")
				} else if strings.HasPrefix(args[i], "--project=") {
					project = strings.TrimPrefix(args[i], "--project=")
					hasProject = true
				}
			}
		}
		// Detect project root and project name (like engram does)
		projectRoot, _ := os.Getwd()
		if gitRoot, err := exec.Command("git", "rev-parse", "--show-toplevel").Output(); err == nil {
			projectRoot = strings.TrimSpace(string(gitRoot))
		}
		engramProject := project
		if !doAll && project == "" && !fromEngram {
			project = filepath.Base(projectRoot) // auto-detect from dir name (only for default bigmem path)
		}
		if fromEngram && !hasProject {
			engramProject = "" // no filter => import all projects
		}
		switch {
		case doStatus:
			st, err := store.SyncStatus(projectRoot)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				return 1
			}
			fmt.Printf("Sync status:\n")
			fmt.Printf("  Dir:      %s\n", st.ExportDir)
			fmt.Printf("  Chunks:   %d\n", st.ChunkCount)
			fmt.Printf("  Entries:  %d\n", st.ObsCount)

		case doImport && fromEngram:
			resolvedDir, rerr := bigmem.ResolveEngramDir(engramDir)
			if rerr != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", rerr)
				return 1
			}
			result, err := store.ImportFromEngram(resolvedDir, engramProject)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				return 1
			}
			fmt.Printf("Imported %d observations (%d chunks, %d skipped) from %s\n", result.ObservationsImported, result.ChunksImported, result.ChunksSkipped, resolvedDir)

		case doImport:
			// Default bigmem transport via dependency-safe import (FileTransport)
			transport := bigmem.NewFileTransport(filepath.Join(projectRoot, ".bigmem"))
			result, err := store.SyncImportDependencySafe(transport)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				return 1
			}
			fmt.Printf("Imported %d observations (%d chunks, %d skipped)\n", result.ObservationsImported, result.ChunksImported, result.ChunksSkipped)

		default:
			// No flags = export (like engram)
			if err := store.SyncExport(project, projectRoot); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				return 1
			}
			fmt.Printf("Exported to %s\n", filepath.Join(projectRoot, ".bigmem"))
		}

	case "graph":
		return bigmemGraphRun(store, args[1:])

	case "version", "--version", "-v":
		fmt.Println("bigmem vdev")
		return 0

	case "help", "--help", "-h":
		return bigmemRun() // re-run to show help (args[0] is "help", won't match)

	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", args[0])
		return 1
	}

	return 0
}

// bigmemGraphRun renders topic_key hierarchy and memory_relations BM25 edges.
func bigmemGraphRun(store *bigmem.Store, args []string) int {
	format := "ascii"
	project := ""
	scope := "project"
	limit := 50

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--project":
			if i+1 < len(args) {
				project = args[i+1]
				i++
			}
		case "--format":
			if i+1 < len(args) {
				format = strings.ToLower(args[i+1])
				i++
			}
		case "--limit":
			if i+1 < len(args) {
				if n, err := strconv.Atoi(args[i+1]); err == nil {
					limit = n
				}
				i++
			}
		case "--scope":
			if i+1 < len(args) {
				scope = strings.ToLower(args[i+1])
				i++
			}
		case "--help", "-h":
			fmt.Fprintln(os.Stderr, "Usage: biggz bigmem graph [--project <name>] [--format dot|ascii|json] [--limit N] [--scope project|all]")
			return 0
		default:
			fmt.Fprintf(os.Stderr, "unknown flag for graph: %s\n", args[i])
			return 1
		}
	}

	if format != "ascii" && format != "dot" && format != "json" {
		fmt.Fprintf(os.Stderr, "error: invalid format %q (dot|ascii|json)\n", format)
		return 1
	}
	if limit <= 0 {
		limit = 50
	}
	if scope == "all" {
		project = ""
	} else if project == "" {
		// default current project detection (git root base or cwd base)
		cwd, _ := os.Getwd()
		projectRoot := cwd
		if out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output(); err == nil {
			projectRoot = strings.TrimSpace(string(out))
		}
		if projectRoot != "" {
			project = filepath.Base(projectRoot)
		}
		// If no observations exist for this project, BuildGraph will return empty
		// and Render will show "No graph data" — gracefully handle empty case below.
	}

	nodes, edges, err := store.BuildGraph(project, limit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: graph: %v\n", err)
		return 1
	}
	// If scope is project and graph is empty due to over-filtering, fall back to all
	// to avoid showing empty when data exists in other projects (helps manual verification).
	if len(nodes) == 0 && project != "" && scope == "project" {
		altNodes, altEdges, _ := store.BuildGraph("", limit)
		if len(altNodes) > 0 || len(altEdges) > 0 {
			nodes, edges = altNodes, altEdges
		}
	}

	switch format {
	case "ascii":
		fmt.Println(bigmem.RenderASCII(nodes, edges))
	case "dot":
		fmt.Println(bigmem.RenderDOT(nodes, edges))
	case "json":
		js, err := bigmem.RenderJSON(nodes, edges)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: json: %v\n", err)
			return 1
		}
		fmt.Println(js)
	}
	return 0
}
