package orchestrator

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

type progressTracker struct {
	mu        sync.Mutex
	out       io.Writer
	target    string
	total     int
	completed int
	started   time.Time
	width     int
}

func newProgressTracker(out io.Writer, target string, total int) *progressTracker {
	if total < 1 {
		total = 1
	}
	return &progressTracker{out: out, target: target, total: total, started: time.Now(), width: 28}
}

func (p *progressTracker) start(step, description string) string {
	p.mu.Lock()
	defer p.mu.Unlock()
	label := fmt.Sprintf("%s/%s", p.target, step)
	p.printLocked("RUN", step, description)
	return label
}

func (p *progressTracker) complete(step, description string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.completed < p.total {
		p.completed++
	}
	p.printLocked("OK ", step, description)
}

func (p *progressTracker) skip(step, description string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.completed < p.total {
		p.completed++
	}
	p.printLocked("SKIP", step, description)
}

func (p *progressTracker) fail(step, description string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.printLocked("FAIL", step, description)
}

func (p *progressTracker) note(step, description string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.printLocked("INFO", step, description)
}

func (p *progressTracker) printLocked(status, step, description string) {
	if p.out == nil || p.out == io.Discard {
		return
	}
	percent := p.completed * 100 / p.total
	filled := p.completed * p.width / p.total
	if filled > p.width {
		filled = p.width
	}
	bar := strings.Repeat("#", filled) + strings.Repeat("-", p.width-filled)
	current := p.completed + 1
	if status == "OK " || status == "SKIP" {
		current = p.completed
	}
	if current > p.total {
		current = p.total
	}
	fmt.Fprintf(p.out, "[%s] [%s] %3d%% step %d/%d %-4s %-5s %s | elapsed=%s\n",
		p.target, bar, percent, current, p.total, step, status, description, formatProgressDuration(time.Since(p.started)))
}

func formatProgressDuration(d time.Duration) string {
	d = d.Round(time.Second)
	if d < 0 {
		d = 0
	}
	h := int(d / time.Hour)
	m := int((d % time.Hour) / time.Minute)
	s := int((d % time.Minute) / time.Second)
	if h > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}
