import { useState, useEffect, useRef } from 'react';
import './App.css';
import { GetConfig, SaveConfig, CaptureAndProcess, GetHistory, SaveCalibration, RemoveRecentModel, FetchAvailableModels } from "../wailsjs/go/main/App";

function App() {
  const [view, setView] = useState("results"); // 'overlay' | 'settings' | 'results'
  const [config, setConfig] = useState(null);
  const [history, setHistory] = useState([]);
  const [isCalibrating, setIsCalibrating] = useState(false);
  const [isProcessing, setIsProcessing] = useState(false);
  const [currentResult, setCurrentResult] = useState("");
  const [activeTab, setActiveTab] = useState("api"); // 'api' | 'tweak'
  const lastShownRef = useRef(0);
  const [scaleFactor, setScaleFactor] = useState(1.0);
  const [recentModels, setRecentModels] = useState([]);
  const [availableModels, setAvailableModels] = useState([]);
  const [showDropdown, setShowDropdown] = useState(false);
  
  // Overlay Selection States
  const [bounds, setBounds] = useState({ x: 0, y: 0, w: 0, h: 0 });
  const [crop, setCrop] = useState({ x: 0, y: 0, w: 0, h: 0 });
  const [isDragging, setIsDragging] = useState(false);
  const [dragStart, setDragStart] = useState({ x: 0, y: 0 });
  
  // Form Settings States
  const [apiProvider, setApiProvider] = useState("openai");
  const [apiUrl, setApiUrl] = useState("http://localhost:20127/v1");
  const [apiKey, setApiKey] = useState("123456789");
  const [modelName, setModelName] = useState("main-combo");
  const [maxTokens, setMaxTokens] = useState(2048);
  const [fontSize, setFontSize] = useState(14);
  const [hotkey, setHotkey] = useState("Ctrl+C");
  const [displayMode, setDisplayMode] = useState("latest");
  const [promptText, setPromptText] = useState("");

  const containerRef = useRef(null);

  // Load configuration and history
  const loadConfiguration = async () => {
    try {
      const cfg = await GetConfig();
      setConfig(cfg);
      setApiProvider(cfg.api_provider || "openai");
      setApiUrl(cfg.api_url || "");
      setApiKey(cfg.api_key || "");
      setModelName(cfg.model_name || "");
      setRecentModels(cfg.recent_models || []);
      setMaxTokens(cfg.max_tokens || 2048);
      setFontSize(cfg.font_size || 14);
      setHotkey(cfg.hotkey || "Ctrl+C");
      setDisplayMode(cfg.display_mode || "latest");
      setPromptText(cfg.prompt || "");
    } catch (e) {
      console.error(e);
    }
  };

  const loadHistory = async () => {
    try {
      const hist = await GetHistory();
      setHistory(hist || []);
      if (hist && hist.length > 0) {
        setCurrentResult(hist[0].content);
      }
    } catch (e) {
      console.error(e);
    }
  };

  const fetchModels = async () => {
    try {
      const models = await FetchAvailableModels();
      setAvailableModels(models || []);
    } catch (e) {
      console.error("Failed to fetch models", e);
    }
  };

  useEffect(() => {
    if (view === "settings") {
      fetchModels();
    }
  }, [view, apiProvider, apiUrl, apiKey]);

  useEffect(() => {
    const handleClickOutside = (e) => {
      if (showDropdown && !e.target.closest('.form-group')) {
        setShowDropdown(false);
      }
    };
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, [showDropdown]);

  useEffect(() => {
    loadConfiguration();
    loadHistory();

    // Listen for crop startup
    const unsubCrop = window.runtime.EventsOn("start-crop", (screenBounds) => {
      setBounds(screenBounds);
      setScaleFactor(screenBounds.scale || 1.0);
      setCrop({ x: 0, y: 0, w: 0, h: 0 });
      setView("overlay");
    });

    // Listen for settings popup (Tab2)
    const unsubSettings = window.runtime.EventsOn("show-tab2", () => {
      setIsCalibrating(false);
      setView("settings");
    });

    // Listen for AI processing status
    const unsubProcessStart = window.runtime.EventsOn("processing-start", () => {
      setIsProcessing(true);
      setCurrentResult("");
      setView("results");
    });

    const unsubProcessComplete = window.runtime.EventsOn("processing-complete", (result) => {
      setIsProcessing(false);
      setCurrentResult(result);
      lastShownRef.current = Date.now();
      loadHistory();
    });

    return () => {
      if (unsubCrop) unsubCrop();
      if (unsubSettings) unsubSettings();
      if (unsubProcessStart) unsubProcessStart();
      if (unsubProcessComplete) unsubProcessComplete();
    };
  }, []);

  // Listen for Escape key to cancel selection overlay
  useEffect(() => {
    const handleKeyDown = (e) => {
      if (e.key === "Escape" && view === "overlay") {
        e.preventDefault();
        // Send cancel capture (width=0, height=0) to Go backend
        CaptureAndProcess(0, 0, 0, 0);
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [view]);

  // Listen for click-outside (blur) on floating result Tab1
  useEffect(() => {
    const handleBlur = () => {
      if (Date.now() - lastShownRef.current < 1000) {
        return;
      }
      if (view === "results" && !isCalibrating && !isProcessing) {
        window.runtime.WindowHide();
      }
    };
    window.addEventListener("blur", handleBlur);
    return () => window.removeEventListener("blur", handleBlur);
  }, [view, isCalibrating, isProcessing]);

  // Handle Drag Selection
  const handleMouseDown = (e) => {
    if (view !== "overlay") return;
    setIsDragging(true);
    setDragStart({ x: e.clientX, y: e.clientY });
    setCrop({ x: e.clientX, y: e.clientY, w: 0, h: 0 });
  };

  const handleMouseMove = (e) => {
    if (!isDragging || view !== "overlay") return;
    const currentX = e.clientX;
    const currentY = e.clientY;
    
    const x = Math.min(dragStart.x, currentX);
    const y = Math.min(dragStart.y, currentY);
    const w = Math.abs(dragStart.x - currentX);
    const h = Math.abs(dragStart.y - currentY);
    
    setCrop({ x, y, w, h });
  };

  const handleMouseUp = () => {
    if (!isDragging || view !== "overlay") return;
    setIsDragging(false);

    // Calculate actual screen coordinates
    // bounds.x, bounds.y represent the virtual screen start (could be negative on multi-monitor)
    const actualX = Math.round(bounds.x + crop.x * scaleFactor);
    const actualY = Math.round(bounds.y + crop.y * scaleFactor);
    const actualW = Math.round(crop.w * scaleFactor);
    const actualH = Math.round(crop.h * scaleFactor);

    // Trigger crop processing in Go
    CaptureAndProcess(actualX, actualY, actualW, actualH);
  };

  // Save Settings Form
  const handleSaveConfig = async (e) => {
    e.preventDefault();
    const updated = {
      ...config,
      api_provider: apiProvider,
      api_url: apiUrl,
      api_key: apiKey,
      model_name: modelName,
      max_tokens: parseInt(maxTokens) || 2048,
      font_size: parseInt(fontSize) || 14,
      hotkey: hotkey,
      display_mode: displayMode,
      prompt: promptText,
    };
    
    const success = await SaveConfig(updated);
    if (success) {
      setConfig(updated);
      // Close settings window
      window.runtime.WindowHide();
    }
  };

  // Trigger Calibration Mode
  const startCalibration = () => {
    setIsCalibrating(true);
    setView("results");
    window.runtime.WindowSetFrameless(true);
    window.runtime.WindowSetSize(config.window_width, config.window_height);
    window.runtime.WindowSetPosition(config.window_x, config.window_y);
  };

  // Save Calibration Data
  const saveCalibrationPos = async () => {
    const pos = await window.runtime.WindowGetPosition();
    const size = await window.runtime.WindowGetSize();
    
    await SaveCalibration(pos.x, pos.y, size.w, size.h);
    
    // Refresh configuration
    await loadConfiguration();
    setIsCalibrating(false);
    
    // Hide window
    window.runtime.WindowHide();
  };

  // Handle Resize Drag in Calibration
  const handleResizeMouseDown = (e) => {
    e.preventDefault();
    e.stopPropagation();
    
    const startWidth = window.innerWidth;
    const startHeight = window.innerHeight;
    const startMouseX = e.clientX;
    const startMouseY = e.clientY;
    
    const onMouseMove = (moveEvent) => {
      const deltaX = moveEvent.clientX - startMouseX;
      const deltaY = moveEvent.clientY - startMouseY;
      const newWidth = Math.max(250, startWidth + deltaX);
      const newHeight = Math.max(200, startHeight + deltaY);
      
      window.runtime.WindowSetSize(newWidth, newHeight);
    };
    
    const onMouseUp = () => {
      document.removeEventListener('mousemove', onMouseMove);
      document.removeEventListener('mouseup', onMouseUp);
    };
    
    document.addEventListener('mousemove', onMouseMove);
    document.addEventListener('mouseup', onMouseUp);
  };

  if (!config && view !== "overlay") {
    return (
      <div className="loading-container">
        <div className="spinner"></div>
        <div>Đang tải cấu hình...</div>
      </div>
    );
  }

  return (
    <div id="App">
      
      {/* 1. OVERLAY SELECTION VIEW */}
      {view === "overlay" && (
        <div 
          className="overlay-container" 
          onMouseDown={handleMouseDown}
          onMouseMove={handleMouseMove}
          onMouseUp={handleMouseUp}
        >
          {crop.w > 0 && crop.h > 0 && (
            <div 
              className="crop-rect"
              style={{
                left: `${crop.x}px`,
                top: `${crop.y}px`,
                width: `${crop.w}px`,
                height: `${crop.h}px`
              }}
            />
          )}
          <div 
            className="overlay-info"
            style={{
              left: `${Math.min(bounds.w - 200, Math.max(20, crop.x)) + 15}px`,
              top: `${Math.min(bounds.h - 100, Math.max(20, crop.y + crop.h)) + 15}px`
            }}
          >
            {crop.w > 0 && crop.h > 0 ? (
              <span>Rộng: {crop.w}px | Cao: {crop.h}px</span>
            ) : (
              <span>Nhấp kéo để khoanh vùng (Esc để hủy)</span>
            )}
          </div>
        </div>
      )}

      {/* 2. TAB2 - SETTINGS VIEW */}
      {view === "settings" && (
        <div className="settings-container">
          <div className="custom-titlebar">
            <span className="titlebar-text">Cài đặt màu sắc - Tinh chỉnh</span>
            <button className="titlebar-close" onClick={() => window.runtime.WindowHide()} aria-label="Đóng Cài đặt">&times;</button>
          </div>
          
          <div className="settings-body">
            <h2 className="settings-header" id="settings-title">Cấu hình Cài đặt</h2>
            
            <div className="tabs-nav" role="tablist">
              <button 
                className={`tab-btn ${activeTab === "api" ? "active" : ""}`}
                onClick={() => setActiveTab("api")}
                role="tab"
                aria-selected={activeTab === "api"}
                id="tab-api"
              >
                API Kết nối
              </button>
              <button 
                className={`tab-btn ${activeTab === "tweak" ? "active" : ""}`}
                onClick={() => setActiveTab("tweak")}
                role="tab"
                aria-selected={activeTab === "tweak"}
                id="tab-tweak"
              >
                Tinh chỉnh & Phím tắt
              </button>
            </div>

            <form className="settings-content" onSubmit={handleSaveConfig}>
              {activeTab === "api" ? (
                <div role="tabpanel" aria-labelledby="tab-api">
                  <div className="form-group">
                    <label htmlFor="api-provider">Nhà Cung Cấp API</label>
                    <select 
                      id="api-provider" 
                      value={apiProvider} 
                      onChange={(e) => setApiProvider(e.target.value)}
                    >
                      <option value="openai">OpenAI Compatible (Ollama, 9router, Local)</option>
                      <option value="gemini">Google Gemini (AI Studio)</option>
                    </select>
                  </div>
                  
                  <div className="form-group">
                    <label htmlFor="api-url">Địa chỉ IP / URL API</label>
                    <input 
                      type="text" 
                      id="api-url" 
                      value={apiUrl} 
                      onChange={(e) => setApiUrl(e.target.value)} 
                      placeholder={apiProvider === "gemini" ? "Mặc định: https://generativelanguage.googleapis.com" : "Ví dụ: http://localhost:20127/v1"}
                    />
                  </div>

                  <div className="form-group">
                    <label htmlFor="api-key">Mã API (API Key)</label>
                    <input 
                      type="password" 
                      id="api-key" 
                      value={apiKey} 
                      onChange={(e) => setApiKey(e.target.value)} 
                      placeholder="Nhập API Key ở đây"
                    />
                  </div>

                  <div className="form-group" style={{ position: "relative" }}>
                    <label htmlFor="model-name">Tên Mô Hình (Model)</label>
                    <div className="input-dropdown-wrapper" style={{ display: "flex", position: "relative", alignItems: "center" }}>
                      <input 
                        type="text" 
                        id="model-name" 
                        value={modelName} 
                        onChange={(e) => setModelName(e.target.value)} 
                        placeholder={apiProvider === "gemini" ? "Ví dụ: gemini-1.5-flash" : "Ví dụ: main-combo"}
                        style={{ paddingRight: "40px", flex: 1 }}
                      />
                      <button 
                        type="button"
                        className="dropdown-arrow-btn"
                        onClick={() => setShowDropdown(!showDropdown)}
                        style={{
                          position: "absolute",
                          right: "10px",
                          background: "none",
                          border: "none",
                          cursor: "pointer",
                          fontSize: "14px",
                          color: "var(--text-muted)",
                          padding: "8px",
                          display: "flex",
                          alignItems: "center",
                          justifyContent: "center"
                        }}
                        aria-label="Chọn Model"
                      >
                        ▼
                      </button>
                    </div>

                    {showDropdown && (
                      <div className="model-dropdown-menu" style={{
                        position: "absolute",
                        top: "100%",
                        left: 0,
                        right: 0,
                        maxHeight: "220px",
                        overflowY: "auto",
                        background: "#ffffff",
                        border: "1px solid var(--border-color)",
                        borderRadius: "8px",
                        boxShadow: "var(--shadow-lg)",
                        zIndex: 1000,
                        marginTop: "4px"
                      }}>
                        {/* 1. Recent Models */}
                        {recentModels.length > 0 && (
                          <div className="dropdown-section">
                            <div className="dropdown-section-title" style={{
                              padding: "6px 12px",
                              fontSize: "11px",
                              fontWeight: "bold",
                              color: "var(--accent-pink)",
                              background: "#fff1f2",
                              borderBottom: "1px solid #ffe4e6"
                            }}>
                              MODEL ĐÃ DÙNG GẦN ĐÂY
                            </div>
                            {recentModels.map((m, idx) => (
                              <div 
                                key={`recent-${idx}`} 
                                className="dropdown-item"
                                style={{
                                  display: "flex",
                                  justifyContent: "space-between",
                                  alignItems: "center",
                                  padding: "8px 12px",
                                  cursor: "pointer",
                                  fontSize: "13px",
                                  borderBottom: "1px solid #f1f5f9"
                                }}
                              >
                                <span 
                                  onClick={() => { setModelName(m); setShowDropdown(false); }}
                                  style={{ flex: 1, color: "var(--text-main)", fontWeight: "500" }}
                                >
                                  {m}
                                </span>
                                <button
                                  type="button"
                                  onClick={async (e) => {
                                    e.stopPropagation();
                                    const updatedCfg = await RemoveRecentModel(m);
                                    setRecentModels(updatedCfg.recent_models || []);
                                  }}
                                  style={{
                                    background: "none",
                                    border: "none",
                                    cursor: "pointer",
                                    color: "var(--text-muted)",
                                    fontSize: "14px",
                                    padding: "2px 6px"
                                  }}
                                  title="Xóa khỏi danh sách"
                                >
                                  &times;
                                </button>
                              </div>
                            ))}
                          </div>
                        )}

                        {/* 2. Available Models */}
                        {availableModels.length > 0 && (
                          <div className="dropdown-section">
                            <div className="dropdown-section-title" style={{
                              padding: "6px 12px",
                              fontSize: "11px",
                              fontWeight: "bold",
                              color: "var(--primary)",
                              background: "#f0f9ff",
                              borderBottom: "1px solid #e0f2fe",
                              borderTop: recentModels.length > 0 ? "1px solid var(--border-color)" : "none"
                            }}>
                              DANH SÁCH MODEL TỪ API
                            </div>
                            {availableModels.map((m, idx) => {
                              if (recentModels.includes(m)) return null;
                              return (
                                <div 
                                  key={`avail-${idx}`} 
                                  className="dropdown-item"
                                  onClick={() => { setModelName(m); setShowDropdown(false); }}
                                  style={{
                                    padding: "8px 12px",
                                    cursor: "pointer",
                                    fontSize: "13px",
                                    color: "var(--text-main)",
                                    borderBottom: "1px solid #f1f5f9"
                                  }}
                                >
                                  {m}
                                </div>
                              );
                            })}
                          </div>
                        )}

                        {recentModels.length === 0 && availableModels.length === 0 && (
                          <div style={{ padding: "12px", fontSize: "13px", color: "var(--text-muted)", textAlign: "center" }}>
                            Không tìm thấy model nào. Hãy nhập tay.
                          </div>
                        )}
                      </div>
                    )}
                  </div>

                  <div className="form-group">
                    <label htmlFor="max-tokens">Trả Về (Max Tokens)</label>
                    <input 
                      type="number" 
                      id="max-tokens" 
                      value={maxTokens} 
                      onChange={(e) => setMaxTokens(e.target.value)}
                    />
                  </div>
                </div>
              ) : (
                <div role="tabpanel" aria-labelledby="tab-tweak">
                  <div className="form-group">
                    <label htmlFor="font-size">Cỡ Chữ Kết Quả (px)</label>
                    <input 
                      type="number" 
                      id="font-size" 
                      value={fontSize} 
                      onChange={(e) => setFontSize(e.target.value)}
                    />
                  </div>

                  <div className="form-group">
                    <label htmlFor="hotkey">Hot Key Chụp Vùng</label>
                    <input 
                      type="text" 
                      id="hotkey" 
                      value={hotkey} 
                      onChange={(e) => setHotkey(e.target.value)}
                      placeholder="Ví dụ: Ctrl+C, Ctrl+Shift+A"
                    />
                  </div>

                  <div className="form-group">
                    <label htmlFor="display-mode">Chế Độ Hiển Thị Tab1</label>
                    <select 
                      id="display-mode" 
                      value={displayMode} 
                      onChange={(e) => setDisplayMode(e.target.value)}
                    >
                      <option value="latest">Chỉ hiển thị kết quả của ảnh mới nhất</option>
                      <option value="all">Hiển thị toàn bộ kết quả trong ngày</option>
                    </select>
                  </div>

                  <div className="form-group">
                    <label htmlFor="prompt-text">Prompt gửi kèm hình ảnh</label>
                    <textarea
                      id="prompt-text"
                      value={promptText}
                      onChange={(e) => setPromptText(e.target.value)}
                      rows={4}
                      style={{
                        width: "100%",
                        padding: "10px 14px",
                        border: "1px solid var(--border-color)",
                        borderRadius: "8px",
                        fontSize: "14px",
                        backgroundColor: "var(--input-bg)",
                        color: "var(--text-main)",
                        outline: "none",
                        resize: "vertical"
                      }}
                    />
                  </div>

                  <div className="form-group" style={{ marginTop: "24px" }}>
                    <label>Vị trí và kích thước Tab1</label>
                    <button 
                      type="button" 
                      className="btn btn-secondary" 
                      onClick={startCalibration}
                      style={{ width: "100%", padding: "12px", background: "#e0f2fe", color: "#0369a1" }}
                      id="btn-calibrate"
                    >
                      Cài đặt Tinh chỉnh kích thước & vị trí
                    </button>
                  </div>
                </div>
              )}

              <div className="settings-footer">
                <button 
                  type="button" 
                  className="btn btn-secondary" 
                  onClick={() => window.runtime.WindowHide()}
                  id="btn-cancel"
                >
                  Hủy bỏ
                </button>
                <button 
                  type="submit" 
                  className="btn btn-primary"
                  id="btn-save"
                >
                  Lưu Cấu hình
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* 3. TAB1 - Floating RESULTS VIEW */}
      {view === "results" && (
        <div 
          ref={containerRef}
          className={`results-container ${isCalibrating ? "calibrating" : ""}`}
          style={{ "--font-size-text": `${fontSize}px` }}
        >
          {isCalibrating && (
            <div className="calibration-notice">
              <p>
                <strong>CHẾ ĐỘ TINH CHỈNH VỊ TRÍ</strong><br/><br/>
                - Kéo thả khung này để di chuyển vị trí hiển thị.<br/>
                - Kéo góc dưới bên phải để thay đổi kích thước.<br/>
                - Bấm nút bên dưới để lưu.
              </p>
              <button 
                className="btn btn-primary" 
                onClick={saveCalibrationPos}
                style={{ background: "var(--accent-pink)" }}
                id="btn-save-calibration"
              >
                Lưu vị trí
              </button>
            </div>
          )}

          {isProcessing ? (
            <div className="loading-container">
              <div className="spinner"></div>
              <div>Đang tải kết quả từ AI...</div>
            </div>
          ) : (
            <div className="results-scroll">
              {displayMode === "latest" ? (
                currentResult ? (
                  <div className="result-card">
                    <div className="result-header">
                      <span>ĐÁP ÁN MỚI NHẤT</span>
                      <span>{history[0]?.timestamp || "Vừa xong"}</span>
                    </div>
                    <div className="result-content">{currentResult}</div>
                  </div>
                ) : (
                  <div className="loading-container">
                    <div>Chưa có dữ liệu. Hãy nhấn {config?.hotkey || "Ctrl+C"} để khoanh vùng câu hỏi.</div>
                  </div>
                )
              ) : (
                history.length > 0 ? (
                  history.map((entry, index) => (
                    <div className="result-card" key={index}>
                      <div className="result-header">
                        <span>CÂU HỎI #{history.length - index}</span>
                        <span>{entry.timestamp}</span>
                      </div>
                      <div className="result-content">{entry.content}</div>
                    </div>
                  ))
                ) : (
                  <div className="loading-container">
                    <div>Chưa có lịch sử kết quả ngày hôm nay.</div>
                  </div>
                )
              )}
            </div>
          )}

          {isCalibrating && (
            <div 
              className="resize-handle" 
              onMouseDown={handleResizeMouseDown}
            />
          )}
        </div>
      )}

    </div>
  );
}

export default App;
