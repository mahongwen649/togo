package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path"
	"strconv"
	"strings"

	"imgtool/server/internal/auth"
	"imgtool/server/internal/channels"
	"imgtool/server/internal/generation"
	"imgtool/server/internal/history"
)

type Dependencies struct {
	Auth          *auth.Service
	Channels      *channels.Service
	History       *history.Service
	Generation    *generation.Service
	FileSigner    FileSigner
	FileReader    FileReader
	FileDeleter   FileDeleter
	SecureCookies bool
	SSO           auth.SSOOptions
}

type FileSigner interface {
	SignedGetURL(objectKey string) (string, error)
	SignedDownloadURL(objectKey string, filename string) (string, error)
}

type FileReader interface {
	ReadObject(objectKey string) ([]byte, error)
}

type FileDeleter interface {
	DeleteObject(objectKey string) error
}

func NewServer(deps Dependencies) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, _ *http.Request) {
		OK(w, map[string]string{"status": "ok"})
	})
	if deps.Auth != nil {
		authHandlers := auth.HTTPHandlers{Service: deps.Auth, SecureCookies: deps.SecureCookies}
		mux.HandleFunc("GET /api/me", func(w http.ResponseWriter, r *http.Request) {
			authHandlers.Me(w, r, OK, Fail)
		})
		mux.HandleFunc("POST /api/auth/login", func(w http.ResponseWriter, r *http.Request) {
			authHandlers.Login(w, r, OK, Fail)
		})
		mux.HandleFunc("POST /api/auth/logout", func(w http.ResponseWriter, r *http.Request) {
			authHandlers.Logout(w, r, OK, Fail)
		})
		mux.HandleFunc("GET /sso", func(w http.ResponseWriter, r *http.Request) {
			ticket := strings.TrimSpace(r.URL.Query().Get("ticket"))
			claims, err := auth.VerifySSOTicket(ticket, deps.SSO)
			if err != nil {
				http.Error(w, "请从主站进入图像生成工具", http.StatusUnauthorized)
				return
			}
			session, _, err := deps.Auth.LoginWithSSO(claims)
			if err != nil {
				Fail(w, http.StatusUnauthorized, "auth", "sso_login_failed", "自动登录失败")
				return
			}
			authHandlers.SetSessionCookie(w, session.ID, deps.Auth.SessionMaxAgeSeconds())
			http.Redirect(w, r, "/", http.StatusFound)
		})
		mux.HandleFunc("GET /api/admin/users", func(w http.ResponseWriter, r *http.Request) {
			if !requireAdmin(w, r, deps.Auth) {
				return
			}
			users, err := deps.Auth.ListUsers()
			if err != nil {
				Fail(w, http.StatusInternalServerError, "auth", "list_users_failed", "用户列表读取失败")
				return
			}
			OK(w, map[string]any{"users": users})
		})
		mux.HandleFunc("POST /api/admin/users", func(w http.ResponseWriter, r *http.Request) {
			if !requireAdmin(w, r, deps.Auth) {
				return
			}
			var input struct {
				Username string `json:"username"`
				Password string `json:"password"`
				Role     string `json:"role"`
			}
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				Fail(w, http.StatusBadRequest, "auth", "invalid_json", "用户请求格式错误")
				return
			}
			user, err := deps.Auth.CreateUser(input.Username, input.Password, input.Role)
			if err != nil {
				Fail(w, http.StatusBadRequest, "auth", "create_user_failed", "用户创建失败")
				return
			}
			OK(w, map[string]any{"user": user})
		})
		mux.HandleFunc("PATCH /api/admin/users/{id}", func(w http.ResponseWriter, r *http.Request) {
			if !requireAdmin(w, r, deps.Auth) {
				return
			}
			var input struct {
				Disabled bool `json:"disabled"`
			}
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				Fail(w, http.StatusBadRequest, "auth", "invalid_json", "用户请求格式错误")
				return
			}
			if err := deps.Auth.SetUserDisabled(r.PathValue("id"), input.Disabled); err != nil {
				Fail(w, http.StatusNotFound, "auth", "user_not_found", "用户不存在")
				return
			}
			OK(w, map[string]bool{"updated": true})
		})
		mux.HandleFunc("POST /api/admin/users/{id}/reset-password", func(w http.ResponseWriter, r *http.Request) {
			if !requireAdmin(w, r, deps.Auth) {
				return
			}
			var input struct {
				Password string `json:"password"`
			}
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				Fail(w, http.StatusBadRequest, "auth", "invalid_json", "用户请求格式错误")
				return
			}
			if err := deps.Auth.ResetPassword(r.PathValue("id"), input.Password); err != nil {
				Fail(w, http.StatusBadRequest, "auth", "reset_password_failed", "密码重置失败")
				return
			}
			OK(w, map[string]bool{"updated": true})
		})
	}
	if deps.Auth != nil && deps.Channels != nil {
		mux.HandleFunc("GET /api/channels", func(w http.ResponseWriter, r *http.Request) {
			user, ok := requireUser(w, r, deps.Auth)
			if !ok {
				return
			}
			items, err := deps.Channels.List(user.ID)
			if err != nil {
				Fail(w, http.StatusInternalServerError, "channels", "list_channels_failed", "供应源读取失败")
				return
			}
			OK(w, map[string]any{"channels": items})
		})
		mux.HandleFunc("POST /api/channels", func(w http.ResponseWriter, r *http.Request) {
			user, ok := requireUser(w, r, deps.Auth)
			if !ok {
				return
			}
			var input channels.CreateInput
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				Fail(w, http.StatusBadRequest, "channels", "invalid_json", "供应源请求格式错误")
				return
			}
			channel, err := deps.Channels.Create(user.ID, input)
			if err != nil {
				Fail(w, http.StatusBadRequest, "channels", "create_channel_failed", "供应源创建失败")
				return
			}
			OK(w, map[string]any{"channel": channel})
		})
		mux.HandleFunc("PATCH /api/channels/{id}", func(w http.ResponseWriter, r *http.Request) {
			user, ok := requireUser(w, r, deps.Auth)
			if !ok {
				return
			}
			var input channels.UpdateInput
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				Fail(w, http.StatusBadRequest, "channels", "invalid_json", "供应源请求格式错误")
				return
			}
			channel, err := deps.Channels.Update(user.ID, r.PathValue("id"), input)
			if err != nil {
				Fail(w, http.StatusNotFound, "channels", "channel_not_found", "供应源不存在")
				return
			}
			OK(w, map[string]any{"channel": channel})
		})
		mux.HandleFunc("DELETE /api/channels/{id}", func(w http.ResponseWriter, r *http.Request) {
			user, ok := requireUser(w, r, deps.Auth)
			if !ok {
				return
			}
			if err := deps.Channels.Delete(user.ID, r.PathValue("id")); err != nil {
				Fail(w, http.StatusNotFound, "channels", "channel_not_found", "供应源不存在")
				return
			}
			OK(w, map[string]bool{"deleted": true})
		})
	}
	if deps.Auth != nil && deps.History != nil {
		mux.HandleFunc("GET /api/history", func(w http.ResponseWriter, r *http.Request) {
			user, ok := requireUser(w, r, deps.Auth)
			if !ok {
				return
			}
			records, err := deps.History.ListHistory(user.ID, 100)
			if err != nil {
				Fail(w, http.StatusInternalServerError, "history", "list_history_failed", "历史记录读取失败")
				return
			}
			OK(w, map[string]any{"records": records})
		})
		mux.HandleFunc("DELETE /api/history/{id}", func(w http.ResponseWriter, r *http.Request) {
			user, ok := requireUser(w, r, deps.Auth)
			if !ok {
				return
			}
			historyID := r.PathValue("id")
			files, err := deps.History.FilesForHistory(user.ID, historyID)
			if err != nil {
				if errors.Is(err, history.ErrNotFound) {
					Fail(w, http.StatusNotFound, "history", "history_not_found", "历史记录不存在")
					return
				}
				Fail(w, http.StatusInternalServerError, "history", "delete_history_failed", "历史记录删除失败")
				return
			}
			if deps.FileDeleter != nil {
				for _, file := range files {
					if err := deps.FileDeleter.DeleteObject(file.ObjectKey); err != nil {
						Fail(w, http.StatusInternalServerError, "storage", "delete_file_failed", "源文件删除失败")
						return
					}
				}
			}
			if err := deps.History.DeleteHistory(user.ID, historyID); err != nil {
				Fail(w, http.StatusInternalServerError, "history", "delete_history_failed", "历史记录删除失败")
				return
			}
			OK(w, map[string]bool{"deleted": true})
		})
		mux.HandleFunc("GET /api/tasks", func(w http.ResponseWriter, r *http.Request) {
			user, ok := requireUser(w, r, deps.Auth)
			if !ok {
				return
			}
			limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
			tasks, err := deps.History.ListTasks(user.ID, r.URL.Query().Get("status"), limit)
			if err != nil {
				Fail(w, http.StatusInternalServerError, "history", "list_tasks_failed", "任务列表读取失败")
				return
			}
			OK(w, map[string]any{"tasks": tasks})
		})
		mux.HandleFunc("GET /api/tasks/{id}", func(w http.ResponseWriter, r *http.Request) {
			user, ok := requireUser(w, r, deps.Auth)
			if !ok {
				return
			}
			task, err := deps.History.GetTask(user.ID, r.PathValue("id"))
			if err != nil {
				Fail(w, http.StatusNotFound, "history", "task_not_found", "任务不存在")
				return
			}
			OK(w, map[string]any{"task": task})
		})
		mux.HandleFunc("POST /api/tasks/{id}/cancel", func(w http.ResponseWriter, r *http.Request) {
			user, ok := requireUser(w, r, deps.Auth)
			if !ok {
				return
			}
			record, err := deps.History.CancelTask(user.ID, r.PathValue("id"))
			if err != nil {
				if errors.Is(err, history.ErrNotFound) {
					Fail(w, http.StatusNotFound, "history", "task_not_found", "运行中任务不存在")
					return
				}
				Fail(w, http.StatusInternalServerError, "history", "cancel_task_failed", "任务取消失败")
				return
			}
			OK(w, map[string]any{"cancelled": true, "record": record})
		})
		if deps.FileSigner != nil {
			mux.HandleFunc("GET /api/files/{id}/signed-url", func(w http.ResponseWriter, r *http.Request) {
				user, ok := requireUser(w, r, deps.Auth)
				if !ok {
					return
				}
				file, err := deps.History.GetFile(user.ID, r.PathValue("id"))
				if err != nil {
					Fail(w, http.StatusNotFound, "history", "file_not_found", "文件不存在")
					return
				}
				url, err := deps.FileSigner.SignedGetURL(file.ObjectKey)
				if err != nil {
					Fail(w, http.StatusInternalServerError, "storage", "sign_url_failed", "文件预览地址生成失败")
					return
				}
				OK(w, map[string]any{"url": url, "file": file})
			})
			mux.HandleFunc("GET /api/files/{id}/download-url", func(w http.ResponseWriter, r *http.Request) {
				user, ok := requireUser(w, r, deps.Auth)
				if !ok {
					return
				}
				file, err := deps.History.GetFile(user.ID, r.PathValue("id"))
				if err != nil {
					Fail(w, http.StatusNotFound, "history", "file_not_found", "文件不存在")
					return
				}
				url, err := deps.FileSigner.SignedDownloadURL(file.ObjectKey, downloadFilename(file.ObjectKey))
				if err != nil {
					Fail(w, http.StatusInternalServerError, "storage", "sign_url_failed", "文件下载地址生成失败")
					return
				}
				OK(w, map[string]any{"url": url, "file": file})
			})
		}
		if deps.FileReader != nil {
			mux.HandleFunc("GET /api/files/{id}/content", func(w http.ResponseWriter, r *http.Request) {
				user, ok := requireUser(w, r, deps.Auth)
				if !ok {
					return
				}
				file, err := deps.History.GetFile(user.ID, r.PathValue("id"))
				if err != nil {
					Fail(w, http.StatusNotFound, "history", "file_not_found", "文件不存在")
					return
				}
				bytes, err := deps.FileReader.ReadObject(file.ObjectKey)
				if err != nil {
					Fail(w, http.StatusInternalServerError, "storage", "read_file_failed", "文件读取失败")
					return
				}
				w.Header().Set("Content-Type", file.MimeType)
				w.Header().Set("Content-Disposition", `inline; filename="`+downloadFilename(file.ObjectKey)+`"`)
				_, _ = w.Write(bytes)
			})
		}
	}
	if deps.Auth != nil && deps.Generation != nil {
		mux.HandleFunc("POST /api/generate/image", func(w http.ResponseWriter, r *http.Request) {
			user, ok := requireUser(w, r, deps.Auth)
			if !ok {
				return
			}
			if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
				if err := r.ParseMultipartForm(12 << 20); err != nil {
					Fail(w, http.StatusBadRequest, "generation", "invalid_form", "生成请求格式错误")
					return
				}
				file, header, err := r.FormFile("inputImage")
				if err != nil {
					Fail(w, http.StatusBadRequest, "generation", "missing_input_image", "图生图需要上传输入图片")
					return
				}
				defer file.Close()
				bytes, err := io.ReadAll(io.LimitReader(file, 10*1024*1024+1))
				if err != nil || len(bytes) == 0 || len(bytes) > 10*1024*1024 {
					Fail(w, http.StatusBadRequest, "generation", "invalid_input_image", "输入图片无效或超过 10 MB")
					return
				}
				count, _ := strconv.Atoi(r.FormValue("count"))
				result, err := deps.Generation.StartImage(user.ID, generation.ImageRequest{
					ChannelID:   r.FormValue("channelId"),
					ModelID:     r.FormValue("modelId"),
					Prompt:      r.FormValue("prompt"),
					Size:        r.FormValue("size"),
					Quality:     r.FormValue("quality"),
					AspectRatio: r.FormValue("aspectRatio"),
					Resolution:  r.FormValue("resolution"),
					Count:       count,
					Capability:  "image-to-image",
					Image:       bytes,
					MimeType:    inputImageMimeType(header.Header.Get("Content-Type"), bytes),
				})
				if err != nil {
					Fail(w, http.StatusBadRequest, "generation", "generate_image_failed", err.Error())
					return
				}
				OK(w, result)
				return
			}
			var input struct {
				ChannelID   string `json:"channelId"`
				ModelID     string `json:"modelId"`
				Prompt      string `json:"prompt"`
				Size        string `json:"size"`
				Quality     string `json:"quality"`
				AspectRatio string `json:"aspectRatio"`
				Resolution  string `json:"resolution"`
				Count       int    `json:"count"`
			}
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				Fail(w, http.StatusBadRequest, "generation", "invalid_json", "生成请求格式错误")
				return
			}
			result, err := deps.Generation.StartImage(user.ID, generation.ImageRequest{
				ChannelID:   input.ChannelID,
				ModelID:     input.ModelID,
				Prompt:      input.Prompt,
				Size:        input.Size,
				Quality:     input.Quality,
				AspectRatio: input.AspectRatio,
				Resolution:  input.Resolution,
				Count:       input.Count,
			})
			if err != nil {
				Fail(w, http.StatusBadRequest, "generation", "generate_image_failed", err.Error())
				return
			}
			OK(w, result)
		})
		mux.HandleFunc("POST /api/generate/text", func(w http.ResponseWriter, r *http.Request) {
			user, ok := requireUser(w, r, deps.Auth)
			if !ok {
				return
			}
			if err := r.ParseMultipartForm(12 << 20); err != nil {
				Fail(w, http.StatusBadRequest, "generation", "invalid_form", "生成请求格式错误")
				return
			}
			file, header, err := r.FormFile("inputImage")
			if err != nil {
				Fail(w, http.StatusBadRequest, "generation", "missing_input_image", "图生文需要上传输入图片")
				return
			}
			defer file.Close()
			bytes, err := io.ReadAll(io.LimitReader(file, 10*1024*1024+1))
			if err != nil || len(bytes) == 0 || len(bytes) > 10*1024*1024 {
				Fail(w, http.StatusBadRequest, "generation", "invalid_input_image", "输入图片无效或超过 10 MB")
				return
			}
			result, err := deps.Generation.GenerateText(user.ID, generation.TextRequest{
				ChannelID: r.FormValue("channelId"),
				ModelID:   r.FormValue("modelId"),
				Prompt:    r.FormValue("prompt"),
				Image:     bytes,
				MimeType:  inputImageMimeType(header.Header.Get("Content-Type"), bytes),
			})
			if err != nil {
				Fail(w, http.StatusBadRequest, "generation", "generate_text_failed", err.Error())
				return
			}
			OK(w, result)
		})
	}
	return mux
}

func downloadFilename(objectKey string) string {
	name := path.Base(objectKey)
	name = strings.TrimSpace(strings.ReplaceAll(name, `"`, ""))
	if name == "" || name == "." || name == "/" {
		return "image.png"
	}
	return name
}

func inputImageMimeType(headerValue string, bytes []byte) string {
	value := normalizeInputImageMimeType(headerValue)
	if isInputImageMimeType(value) {
		return value
	}
	detected := normalizeInputImageMimeType(http.DetectContentType(bytes))
	if isInputImageMimeType(detected) {
		return detected
	}
	return value
}

func normalizeInputImageMimeType(value string) string {
	value = strings.ToLower(strings.TrimSpace(strings.Split(value, ";")[0]))
	if value == "image/jpg" {
		return "image/jpeg"
	}
	return value
}

func isInputImageMimeType(value string) bool {
	return value == "image/png" || value == "image/jpeg" || value == "image/webp"
}

func requireAdmin(w http.ResponseWriter, r *http.Request, service *auth.Service) bool {
	user, ok := requireUser(w, r, service)
	if !ok {
		return false
	}
	if user.Role != auth.RoleAdmin {
		Fail(w, http.StatusForbidden, "auth", "forbidden", "需要管理员权限")
		return false
	}
	return true
}

func requireUser(w http.ResponseWriter, r *http.Request, service *auth.Service) (auth.User, bool) {
	cookie, err := r.Cookie(auth.CookieName)
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		Fail(w, http.StatusUnauthorized, "auth", "unauthenticated", "请先登录")
		return auth.User{}, false
	}
	user, err := service.UserForSession(cookie.Value)
	if err != nil {
		Fail(w, http.StatusUnauthorized, "auth", "unauthenticated", "请先登录")
		return auth.User{}, false
	}
	return user, true
}
