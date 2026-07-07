package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kbinani/screenshot"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// Config represents application settings
type Config struct {
	APIProvider  string `json:"api_provider"`
	APIURL       string `json:"api_url"`
	APIKey       string `json:"api_key"`
	ModelName    string `json:"model_name"`
	MaxTokens    int    `json:"max_tokens"`
	FontSize     int    `json:"font_size"`
	Hotkey       string `json:"hotkey"`
	DisplayMode  string `json:"display_mode"`
	Prompt       string `json:"prompt"`
	WindowWidth  int    `json:"window_width"`
	WindowHeight int    `json:"window_height"`
	WindowX      int    `json:"window_x"`
	WindowY      int    `json:"window_y"`
}

// HistoryEntry represents a past AI result
type HistoryEntry struct {
	Timestamp string `json:"timestamp"`
	Content   string `json:"content"`
}

// App struct
type App struct {
	ctx          context.Context
	config       Config
	isCapturing  bool
}

func writeLog(format string, args ...interface{}) {
	logPath := getConfigPath()
	dir := filepath.Dir(logPath)
	debugLogPath := filepath.Join(dir, "debug.log")
	
	f, err := os.OpenFile(debugLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	msg := fmt.Sprintf(format, args...)
	f.WriteString(fmt.Sprintf("[%s] %s\n", timestamp, msg))
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.LoadConfig()
}

// getConfigPath resolves the correct path to config.json
func getConfigPath() string {
	// Try "../Config/config.json" first for development (running from Project/)
	if _, err := os.Stat("../Config/config.json"); err == nil {
		return "../Config/config.json"
	}
	// Try "Config/config.json" for production (running from root)
	if _, err := os.Stat("Config/config.json"); err == nil {
		return "Config/config.json"
	}
	// Fallback to executable folder
	if exePath, err := os.Executable(); err == nil {
		dir := filepath.Dir(exePath)
		return filepath.Join(dir, "Config", "config.json")
	}
	return "Config/config.json"
}

// getSaveDir resolves the correct path to the Save/ directory
func getSaveDir() string {
	configPath := getConfigPath()
	dir := filepath.Dir(configPath)
	saveDir := filepath.Join(dir, "Save")
	os.MkdirAll(saveDir, 0755)
	return saveDir
}

// GetConfig returns the current configuration
func (a *App) GetConfig() Config {
	return a.config
}

// LoadConfig loads configuration from file
func (a *App) LoadConfig() Config {
	path := getConfigPath()
	
	// Create default directory if it doesn't exist
	dir := filepath.Dir(path)
	os.MkdirAll(dir, 0755)

	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Default config
		a.config = Config{
			APIProvider:  "openai",
			APIURL:       "http://localhost:20127/v1",
			APIKey:       "123456789",
			ModelName:    "main-combo",
			MaxTokens:    2048,
			FontSize:     14,
			Hotkey:       "Ctrl+C",
			DisplayMode:  "latest",
			Prompt:       "Hãy trả lời câu hỏi trong ảnh, giải thích ngắn gọn trong hai câu, tự nhiên, dễ hiểu. Luôn trả lời bằng tiếng Việt.",
			WindowWidth:  400,
			WindowHeight: 550,
			WindowX:      100,
			WindowY:      100,
		}
		a.SaveConfig(a.config)
	} else {
		data, err := os.ReadFile(path)
		if err == nil {
			var cfg Config
			if err := json.Unmarshal(data, &cfg); err == nil {
				a.config = cfg
			}
		}
	}
	return a.config
}

// SaveConfig saves configuration to file
func (a *App) SaveConfig(cfg Config) bool {
	a.config = cfg
	path := getConfigPath()
	
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return false
	}
	
	err = os.WriteFile(path, data, 0644)
	if err != nil {
		return false
	}

	// Trigger hotkey update in main
	runtime.EventsEmit(a.ctx, "hotkey-changed", cfg.Hotkey)
	return true
}

// TriggerCapture switches the window to fullscreen crop overlay mode
func (a *App) TriggerCapture() {
	if a.isCapturing {
		return
	}
	a.isCapturing = true

	// Hide main window first
	runtime.WindowHide(a.ctx)

	// Sleep briefly to ensure window is hidden
	time.Sleep(100 * time.Millisecond)

	// Get total virtual screen bounds (covering all monitors)
	bounds := image.Rect(0, 0, 0, 0)
	for i := 0; i < screenshot.NumActiveDisplays(); i++ {
		bounds = bounds.Union(screenshot.GetDisplayBounds(i))
	}

	// Emit start crop event to Frontend
	runtime.EventsEmit(a.ctx, "start-crop", map[string]int{
		"x": bounds.Min.X,
		"y": bounds.Min.Y,
		"w": bounds.Dx(),
		"h": bounds.Dy(),
	})

	// Make window fullscreen, and position it to cover all monitors
	runtime.WindowSetPosition(a.ctx, bounds.Min.X, bounds.Min.Y)
	runtime.WindowSetSize(a.ctx, bounds.Dx(), bounds.Dy())
	runtime.WindowShow(a.ctx)
}

// CaptureAndProcess captures the specified screen region, sends it to AI, saves the result, and shows Tab1
func (a *App) CaptureAndProcess(x, y, w, h int) string {
	a.isCapturing = false
	writeLog("CaptureAndProcess: x=%d, y=%d, w=%d, h=%d", x, y, w, h)

	// Hide fullscreen window immediately
	runtime.WindowHide(a.ctx)

	if w <= 5 || h <= 5 {
		// Selection too small, cancel
		writeLog("CaptureAndProcess: selection too small, cancelling")
		a.restoreMainWindow(false)
		return "CANCELLED"
	}

	// Capture the actual pixels on the screen at this moment
	rect := image.Rect(x, y, x+w, y+h)
	img, err := screenshot.CaptureRect(rect)
	if err != nil {
		writeLog("CaptureAndProcess: error capturing screen: %v", err)
		a.restoreMainWindow(true)
		return fmt.Sprintf("Error capturing screen: %v", err)
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		writeLog("CaptureAndProcess: error encoding image: %v", err)
		a.restoreMainWindow(true)
		return fmt.Sprintf("Error encoding image: %v", err)
	}

	// Send processing state to frontend
	runtime.EventsEmit(a.ctx, "processing-start", nil)

	// Restore window to Tab1 position/size but keep it hidden until result is back
	a.restoreMainWindow(false)

	// Call AI API in a background goroutine to avoid blocking the frontend
	go func() {
		writeLog("CaptureAndProcess background goroutine started")
		result, err := a.sendToAI(buf.Bytes())
		if err != nil {
			writeLog("CaptureAndProcess AI error: %v", err)
			result = fmt.Sprintf("Lỗi kết nối AI: %v", err)
		} else {
			writeLog("CaptureAndProcess AI success, saving result")
			// Save to daily history
			a.saveResult(result)
		}

		// Send result to frontend
		writeLog("CaptureAndProcess: emitting processing-complete")
		runtime.EventsEmit(a.ctx, "processing-complete", result)

		// Show Tab1 window at calibrated position
		writeLog("CaptureAndProcess: showing main window")
		runtime.WindowShow(a.ctx)
	}()

	return "PROCESSING"
}

// restoreMainWindow resets the Wails window back to the Tab1 floating dimensions
func (a *App) restoreMainWindow(show bool) {
	bounds := image.Rect(0, 0, 0, 0)
	for i := 0; i < screenshot.NumActiveDisplays(); i++ {
		bounds = bounds.Union(screenshot.GetDisplayBounds(i))
	}

	x := a.config.WindowX
	y := a.config.WindowY
	w := a.config.WindowWidth
	h := a.config.WindowHeight

	// Validate if the window is inside the virtual screen bounds.
	if x < bounds.Min.X || x > bounds.Max.X-50 || y < bounds.Min.Y || y > bounds.Max.Y-50 {
		// Reset to default/center of primary monitor
		primaryBounds := screenshot.GetDisplayBounds(0)
		x = primaryBounds.Min.X + (primaryBounds.Dx()-w)/2
		y = primaryBounds.Min.Y + (primaryBounds.Dy()-h)/2
		writeLog("restoreMainWindow: Window position (%d, %d) was off-screen. Resetting to (%d, %d)", a.config.WindowX, a.config.WindowY, x, y)
	} else {
		writeLog("restoreMainWindow: Position (%d, %d) is valid within bounds: Min=(%d, %d) Max=(%d, %d)", x, y, bounds.Min.X, bounds.Min.Y, bounds.Max.X, bounds.Max.Y)
	}

	runtime.WindowSetSize(a.ctx, w, h)
	runtime.WindowSetPosition(a.ctx, x, y)
	if show {
		runtime.WindowShow(a.ctx)
	}
}

// SaveCalibration saves the calibrated position and size of Tab1
func (a *App) SaveCalibration(x, y, w, h int) {
	a.config.WindowX = x
	a.config.WindowY = y
	a.config.WindowWidth = w
	a.config.WindowHeight = h
	a.SaveConfig(a.config)
}

// GetHistory reads the current day's log file and returns past results
func (a *App) GetHistory() []HistoryEntry {
	now := time.Now()
	fileName := fmt.Sprintf("%02d-%02d-%d.txt", now.Day(), now.Month(), now.Year())
	filePath := filepath.Join(getSaveDir(), fileName)

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return []HistoryEntry{}
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return []HistoryEntry{}
	}

	// Split log blocks by "=========================================\n"
	blocks := strings.Split(string(data), "=========================================\n")
	var entries []HistoryEntry

	for _, block := range blocks {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}

		// Split timestamp and content
		lines := strings.SplitN(block, "\n-----------------------------------------\n", 2)
		if len(lines) == 2 {
			timestamp := strings.TrimSpace(lines[0])
			timestamp = strings.Replace(timestamp, "[Thời gian: ", "", 1)
			timestamp = strings.Replace(timestamp, "]", "", 1)

			content := strings.TrimSpace(lines[1])
			entries = append(entries, HistoryEntry{
				Timestamp: timestamp,
				Content:   content,
			})
		}
	}

	// Reverse array to show latest first
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}

	return entries
}

// saveResult appends the result to the daily log file
func (a *App) saveResult(text string) error {
	now := time.Now()
	fileName := fmt.Sprintf("%02d-%02d-%d.txt", now.Day(), now.Month(), now.Year())
	filePath := filepath.Join(getSaveDir(), fileName)

	entry := fmt.Sprintf("=========================================\n[Thời gian: %02d:%02d:%02d]\n-----------------------------------------\n%s\n\n",
		now.Hour(), now.Minute(), now.Second(), text)

	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(entry)
	return err
}

// sendToAI performs the API call to Gemini or OpenAI-compatible proxy
func (a *App) sendToAI(imgBytes []byte) (string, error) {
	writeLog("sendToAI: APIProvider=%s, APIURL=%s, ModelName=%s, KeyLen=%d", a.config.APIProvider, a.config.APIURL, a.config.ModelName, len(a.config.APIKey))
	base64Img := base64.StdEncoding.EncodeToString(imgBytes)

	var reqUrl string
	var reqBody []byte
	var err error

	if a.config.APIProvider == "gemini" {
		url := a.config.APIURL
		if url == "" {
			url = "https://generativelanguage.googleapis.com"
		}
		reqUrl = fmt.Sprintf("%s/v1beta/models/%s:generateContent?key=%s", url, a.config.ModelName, a.config.APIKey)

		body := map[string]interface{}{
			"contents": []map[string]interface{}{
				{
					"parts": []map[string]interface{}{
						{"text": a.config.Prompt},
						{
							"inlineData": map[string]string{
								"mimeType": "image/png",
								"data":     base64Img,
							},
						},
					},
				},
			},
		}
		reqBody, err = json.Marshal(body)
	} else {
		reqUrl = a.config.APIURL
		if !strings.HasSuffix(reqUrl, "/chat/completions") {
			if strings.HasSuffix(reqUrl, "/") {
				reqUrl += "chat/completions"
			} else {
				reqUrl += "/chat/completions"
			}
		}

		body := map[string]interface{}{
			"model": a.config.ModelName,
			"messages": []map[string]interface{}{
				{
					"role": "user",
					"content": []map[string]interface{}{
						{
							"type": "text",
							"text": a.config.Prompt,
						},
						{
							"type": "image_url",
							"image_url": map[string]string{
								"url": fmt.Sprintf("data:image/png;base64,%s", base64Img),
							},
						},
					},
				},
			},
			"max_tokens": a.config.MaxTokens,
			"stream":     false,
		}
		reqBody, err = json.Marshal(body)
	}

	if err != nil {
		writeLog("sendToAI: marshal body error: %v", err)
		return "", err
	}

	writeLog("sendToAI: POST to %s", reqUrl)
	req, err := http.NewRequest("POST", reqUrl, bytes.NewBuffer(reqBody))
	if err != nil {
		writeLog("sendToAI: create request error: %v", err)
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if a.config.APIProvider != "gemini" && a.config.APIKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", a.config.APIKey))
	}

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		writeLog("sendToAI: client Do error: %v", err)
		return "", err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		writeLog("sendToAI: read body error: %v", err)
		return "", err
	}

	writeLog("sendToAI: status code = %d, response body length = %d", resp.StatusCode, len(respBytes))

	if resp.StatusCode != 200 {
		writeLog("sendToAI: API error response: %s", string(respBytes))
		return "", fmt.Errorf("API error (%d): %s", resp.StatusCode, string(respBytes))
	}

	if a.config.APIProvider == "gemini" {
		var geminiResp struct {
			Candidates []struct {
				Content struct {
					Parts []struct {
						Text string `json:"text"`
					} `json:"parts"`
				} `json:"content"`
			} `json:"candidates"`
		}
		if err := json.Unmarshal(respBytes, &geminiResp); err != nil {
			writeLog("sendToAI: gemini unmarshal error: %v", err)
			return "", err
		}
		if len(geminiResp.Candidates) > 0 && len(geminiResp.Candidates[0].Content.Parts) > 0 {
			resText := geminiResp.Candidates[0].Content.Parts[0].Text
			writeLog("sendToAI success: %s", resText)
			return resText, nil
		}
	} else {
		var openAIResp struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(respBytes, &openAIResp); err != nil {
			writeLog("sendToAI: openai unmarshal error: %v", err)
			return "", err
		}
		if len(openAIResp.Choices) > 0 {
			resText := openAIResp.Choices[0].Message.Content
			writeLog("sendToAI success: %s", resText)
			return resText, nil
		}
	}

	writeLog("sendToAI: no valid response content received")
	return "", fmt.Errorf("không nhận được phản hồi hợp lệ từ AI")
}

// ExitApp closes the application
func (a *App) ExitApp() {
	runtime.Quit(a.ctx)
}
