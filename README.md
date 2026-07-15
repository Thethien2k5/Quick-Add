# Quick-Add AI Helper (SystemSettings)

Ứng dụng desktop tiện ích (phát triển bằng **Wails v2** + **Go** + **React/Vite**) giúp chụp nhanh một vùng màn hình bằng phím tắt, tự động gửi ảnh chụp tới AI (API tương thích OpenAI) để giải thích, dịch hoặc giải đáp câu hỏi, và hiển thị kết quả trên một cửa sổ nổi luôn nổi trên cùng (Always on Top).

Dự án này chạy ngầm dưới khay hệ thống (Stealth Mode/System Tray) và hỗ trợ phím tắt toàn hệ thống (Global Hotkey).

---

## 🚀 Các tính năng chính

1. **Chụp ảnh màn hình nhanh bằng phím tắt (Global Hotkey)**:
   - Nhấn phím tắt (mặc định là `Ctrl+C`, có thể tùy chỉnh) để kích hoạt chế độ quét/chụp màn hình.
   - Giao diện chụp màn hình hiển thị toàn màn hình, cho phép kéo và thả chuột để chọn vùng cần phân tích.

2. **Tự động xử lý bằng AI**:
   - Vùng chụp ảnh màn hình được mã hóa sang định dạng Base64 và gửi tới API OpenAI-compatible (hỗ trợ OpenAI, các proxy trung gian hoặc các local gateway như **AI Local Gateway**).
   - Tùy chỉnh Prompt hệ thống cho AI (ví dụ: yêu cầu giải thích ngắn gọn bằng tiếng Việt, hướng dẫn chi tiết, v.v.).

3. **Giao diện hiển thị dạng nổi (Floating Window)**:
   - Sau khi AI xử lý xong, hiển thị kết quả trên một cửa sổ nổi, không viền (frameless), luôn hiển thị trên cùng (Always on top).
   - Có thể điều chỉnh kích thước và vị trí cửa sổ và lưu lại cấu hình đó tự động (Calibration).

4. **Lưu lịch sử hàng ngày**:
   - Các phản hồi từ AI được tự động lưu lại vào file text phân chia theo ngày tại thư mục `Config/Save/DD-MM-YYYY.txt`.
   - Xem lại lịch sử các câu trả lời trực tiếp trên giao diện ứng dụng.

5. **Chạy ẩn (Stealth Mode) & Khay hệ thống (System Tray)**:
   - Ứng dụng không hiển thị biểu tượng dưới Taskbar khi hoạt động, hoàn toàn ẩn mình.
   - Biểu tượng dưới System Tray cho phép người dùng click chuột phải để:
     - *Edit*: Mở giao diện cài đặt (Tab 2) ở dạng cửa sổ rộng (`800x550`), cho phép chỉnh sửa API URL, API Key, Model, Prompt, Hotkey, Font Size...
     - *Exit*: Thoát hoàn toàn ứng dụng.

---

## 💻 Yêu cầu hệ thống & Môi trường

- **Hệ điều hành**: Windows (Hỗ trợ tốt nhất các hàm API Windows cho phím tắt và ẩn taskbar).
- **Môi trường phát triển**:
  - **Golang**: Phiên bản 1.20 trở lên ([Tải Go](https://go.dev/dl/)).
  - **Node.js**: Phiên bản LTS ([Tải Node.js](https://nodejs.org/)).
  - **Wails CLI**: Phiên bản v2. Cài đặt bằng lệnh:
    ```bash
    go install github.com/wailsapp/wails/v2/cmd/wails@latest
    ```
    *(Đảm bảo thư mục `%USERPROFILE%\go\bin` đã được thêm vào PATH)*.

---

## 🛠️ Hướng dẫn cài đặt & Biên dịch (Build to EXE)

### Bước 1: Khởi chạy ở chế độ phát triển (Development)
Để chạy thử và chỉnh sửa mã nguồn có hỗ trợ hot-reload:
1. Mở terminal và di chuyển vào thư mục `Project`:
   ```bash
   cd Project
   ```
2. Chạy lệnh phát triển của Wails:
   ```bash
   wails dev
   ```

### Bước 2: Biên dịch ứng dụng thành file .exe (Build Production)
Để tạo ra file chạy độc lập `SystemSettings.exe`:
1. Trong thư mục `Project`, chạy lệnh biên dịch:
   ```bash
   wails build
   ```
2. Sau khi biên dịch thành công, file thực thi (.exe) sẽ được tạo ra tại:
   `Project/build/bin/SystemSettings.exe`
3. Copy file `SystemSettings.exe` này ra thư mục gốc của dự án (cùng cấp với thư mục `Config/` và file `hướng dẫn.txt`) để khởi chạy.

---

## 📁 Cấu trúc dự án

```
├── Project/            # Thư mục chứa mã nguồn Wails chính
│   ├── app.go          # Logic ứng dụng Go backend (Chụp màn hình, Gọi API AI, cấu hình)
│   ├── main.go         # Entrypoint, cấu hình System Tray, Stealth Mode và đăng ký Hotkey
│   ├── go.mod          # Khai báo Go dependencies
│   ├── wails.json      # Cấu hình dự án Wails (Output: SystemSettings.exe)
│   ├── build/          # Các asset build (icon, manifest, file bin đầu ra)
│   └── frontend/       # Giao diện React + Vite + TypeScript (UI hiển thị kết quả và cấu hình)
├── Config/             # Cấu hình cục bộ và dữ liệu sinh ra khi chạy (Không đẩy lên Git)
│   ├── config.json     # Chứa API Key, Model, Prompt, Hotkey và vị trí cửa sổ
│   ├── debug.log       # File log debug của ứng dụng
│   └── Save/           # Thư mục lưu lịch sử câu trả lời hàng ngày dưới dạng file text
├── SystemSettings.exe  # File thực thi sau khi biên dịch và copy ra ngoài (Chạy ứng dụng bằng file này)
└── hướng dẫn.txt       # Tài liệu hướng dẫn cài đặt và build nhanh
```

---

## ⚙️ Cấu hình API và Phím tắt

Mở cài đặt bằng cách chuột phải vào icon ở System Tray -> Chọn **Edit**.
- **API URL**: Địa chỉ API endpoint (ví dụ: `https://api.openai.com/v1` hoặc `http://localhost:8787/v1` nếu kết nối thông qua **AI Local Gateway**).
- **API Key**: API key của bạn.
- **Model Name**: Tên mô hình sử dụng (ví dụ: `gpt-4o-mini`, `gemini-2.5-flash`...).
- **Hotkey**: Phím tắt chụp màn hình (ví dụ: `Ctrl+C`, `Ctrl+Shift+A`...).
- **Prompt**: Chỉ dẫn dành cho AI khi phân tích ảnh chụp (ví dụ: yêu cầu giải thích, dịch thuật...).

---

## 📄 Giấy phép

Phát triển bởi **Thethien2k5** (thienobita0203@gmail.com).
