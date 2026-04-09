package proxy

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/llmate/gateway/internal/models"
)

// UsageInfo holds token usage merged from SSE chunks before [DONE].
type UsageInfo struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
	CachedTokens     int `json:"cached_tokens"`
	// CachedTokensReported is true if any chunk included prompt_tokens_details (value may be 0).
	CachedTokensReported bool `json:"-"`
}

// mergeStreamUsage applies one chunk's usage. Token counts always follow the latest chunk;
// cached token count is preserved when a later chunk omits prompt_tokens_details.
func mergeStreamUsage(current *UsageInfo, promptTokens, completionTokens, totalTokens int, cachedDetails *int) *UsageInfo {
	if current == nil {
		current = &UsageInfo{}
	}
	next := *current
	next.PromptTokens = promptTokens
	next.CompletionTokens = completionTokens
	next.TotalTokens = totalTokens
	if cachedDetails != nil {
		next.CachedTokens = *cachedDetails
		next.CachedTokensReported = true
	}
	return &next
}

// streamBufferEntry is one buffered SSE data line plus optional OpenAI-style assistant text delta.
type streamBufferEntry struct {
	raw   string
	delta string
}

// StreamingBuffer accumulates raw SSE data: lines with a configurable max size.
// When adding a chunk would exceed maxSize, oldest chunks are evicted (rolling window).
type StreamingBuffer struct {
	mu      sync.Mutex
	entries []streamBufferEntry
	maxSize int
	current int
	evicted bool // true if any line was dropped due to the rolling window
}

func NewStreamingBuffer(maxSize int) *StreamingBuffer {
	return &StreamingBuffer{
		entries: make([]streamBufferEntry, 0),
		maxSize: maxSize,
	}
}

func (b *StreamingBuffer) Add(rawLine, contentDelta string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	lineBytes := len(rawLine)
	for b.current+lineBytes > b.maxSize && len(b.entries) > 0 {
		b.evicted = true
		b.current -= len(b.entries[0].raw)
		b.entries = b.entries[1:]
	}
	b.entries = append(b.entries, streamBufferEntry{raw: rawLine, delta: contentDelta})
	b.current += lineBytes
}

// GetAll returns a copy of buffered entries and whether older lines were dropped (prefix lost).
func (b *StreamingBuffer) GetAll() ([]streamBufferEntry, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	result := make([]streamBufferEntry, len(b.entries))
	copy(result, b.entries)
	return result, b.evicted
}

func takeStreamingChunks(buffer *StreamingBuffer) ([]streamBufferEntry, bool) {
	if buffer == nil {
		return nil, false
	}
	return buffer.GetAll()
}

// extractOpenAIContentDelta returns concatenated choices[].delta.content from an SSE JSON payload.
// rewriteSSEDataLineForClientModel replaces the JSON "model" field in an SSE data line when
// present. Used when the client routed via a gateway alias so streamed chunks match the
// requested name. Returns line unchanged if it is not a rewriteable data line.
func rewriteSSEDataLineForClientModel(line, clientModel string) string {
	const prefix = "data:"
	if !strings.HasPrefix(line, prefix) {
		return line
	}
	payload := strings.TrimSpace(line[len(prefix):])
	if payload == "" || payload == "[DONE]" {
		return line
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal([]byte(payload), &obj); err != nil {
		return line
	}
	if _, ok := obj["model"]; !ok {
		return line
	}
	enc, err := json.Marshal(clientModel)
	if err != nil {
		return line
	}
	obj["model"] = json.RawMessage(enc)
	out, err := json.Marshal(obj)
	if err != nil {
		return line
	}
	return prefix + " " + string(out)
}

func extractOpenAIContentDelta(payload string) string {
	var delta struct {
		Choices []struct {
			Delta struct {
				Content string `json:"content"`
			} `json:"delta"`
		} `json:"choices"`
	}
	if err := json.Unmarshal([]byte(payload), &delta); err != nil {
		return ""
	}
	var b strings.Builder
	for _, c := range delta.Choices {
		b.WriteString(c.Delta.Content)
	}
	return b.String()
}

// proxyStreaming copies an SSE stream from backendResp to w line by line.
// It measures time-to-first-token (TTFT), parses usage from the final chunk,
// and optionally reconstructs response body from content deltas and buffers SSE chunks.
// When requestedViaAlias is true, JSON object data lines with a "model" field are rewritten
// to clientModel before sending to the client (and before buffering).
// The caller retains ownership of backendResp.Body (does not close it here).
func (h *Handler) proxyStreaming(w http.ResponseWriter, backendResp *http.Response, startTime time.Time, logConfig map[string]string, clientModel string, requestedViaAlias bool) (usage *UsageInfo, ttftMs *int, reconstructedBody string, chunks []streamBufferEntry, chunksPrefixDropped bool, err error) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, nil, "", nil, false, fmt.Errorf("response writer does not support flushing (streaming impossible)")
	}

	scanner := bufio.NewScanner(backendResp.Body)
	// Default scanner buffer is 64 KiB which can be too small for large SSE chunks.
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	// Response body reconstruction: always accumulate content deltas.
	var bodyBuilder strings.Builder
	respMax := getConfigInt(logConfig, "response_body_max_bytes", models.DefaultResponseBodyMaxBytes)
	bodyCapReached := false

	// Optional chunk-level tracking.
	trackStreaming := getConfigBool(logConfig, "track_streaming", false)
	var buffer *StreamingBuffer
	if trackStreaming {
		bufSize := getConfigInt(logConfig, "streaming_buffer_size", models.DefaultStreamingBufferSize)
		buffer = NewStreamingBuffer(bufSize)
	}

	var firstTokenSeen bool

	for scanner.Scan() {
		line := scanner.Text()
		// Trim CR so \r\n line endings work the same as \n.
		line = strings.TrimRight(line, "\r")

		lineToWrite := line
		if requestedViaAlias {
			lineToWrite = rewriteSSEDataLineForClientModel(line, clientModel)
		}

		// Forward every line to the client including blank lines (SSE protocol requires them).
		if _, writeErr := fmt.Fprintf(w, "%s\n", lineToWrite); writeErr != nil {
			return usage, ttftMs, "", nil, false, fmt.Errorf("writing to client: %w", writeErr)
		}
		flusher.Flush()

		if !strings.HasPrefix(line, "data:") {
			continue
		}

		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "[DONE]" {
			entries, prefixDropped := takeStreamingChunks(buffer)
			return usage, ttftMs, bodyBuilder.String(), entries, prefixDropped, nil
		}
		if payload == "" {
			continue
		}

		// Record TTFT on the first non-empty, non-[DONE] data line.
		if !firstTokenSeen {
			ms := int(time.Since(startTime).Milliseconds())
			ttftMs = &ms
			firstTokenSeen = true
		}

		// Attempt to extract usage from this chunk; last occurrence before [DONE] wins.
		var chunk struct {
			Usage *struct {
				PromptTokens        int `json:"prompt_tokens"`
				CompletionTokens    int `json:"completion_tokens"`
				TotalTokens         int `json:"total_tokens"`
				PromptTokensDetails *struct {
					CachedTokens int `json:"cached_tokens"`
				} `json:"prompt_tokens_details,omitempty"`
			} `json:"usage"`
		}
		if jsonErr := json.Unmarshal([]byte(payload), &chunk); jsonErr == nil && chunk.Usage != nil {
			u := chunk.Usage
			var cachedPtr *int
			if u.PromptTokensDetails != nil {
				v := u.PromptTokensDetails.CachedTokens
				cachedPtr = &v
			}
			usage = mergeStreamUsage(usage, u.PromptTokens, u.CompletionTokens, u.TotalTokens, cachedPtr)
		}

		deltaText := extractOpenAIContentDelta(payload)

		if !bodyCapReached && deltaText != "" {
			bodyBuilder.WriteString(deltaText)
			if respMax > 0 && bodyBuilder.Len() > respMax {
				bodyCapReached = true
			}
		}

		if buffer != nil {
			buffer.Add(lineToWrite, deltaText)
		}
	}

	if scanErr := scanner.Err(); scanErr != nil {
		return usage, ttftMs, "", nil, false, fmt.Errorf("reading stream: %w", scanErr)
	}

	entries, prefixDropped := takeStreamingChunks(buffer)
	return usage, ttftMs, bodyBuilder.String(), entries, prefixDropped, nil
}
