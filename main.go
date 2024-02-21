package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/smtp"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
)

type Config struct {
	Email struct {
		IMAPServer string `json:"imapServer"`
		Username   string `json:"username"`
		Password   string `json:"password"`
		Mailbox    string `json:"mailbox"`
	} `json:"email"`
	Webhook struct {
		WeComUrl string `json:"weComUrl"`
	} `json:"webhook"`
	Llm struct {
		Host   string `json:"host"`
		Api    string `json:"api"`
		Token  string `json:"token"`
		Model  string `json:"model"`
		Prompt string `json:"prompt"`
	} `json:"llm"`
}

func loadConfig(filePath string) Config {
	var config Config
	configFile, err := os.ReadFile(filePath)
	if err != nil {
		log.Fatal("Cannot load config:", err)
	}
	err = json.Unmarshal(configFile, &config)
	if err != nil {
		log.Fatal("Error parsing config json:", err)
	}

	overrideConfigFromEnv(&config)

	return config
}

func overrideConfigFromEnv(config *Config) {
	if envVar := os.Getenv("IMAP_SERVER"); envVar != "" {
		config.Email.IMAPServer = envVar
	}
	if envVar := os.Getenv("USERNAME"); envVar != "" {
		config.Email.Username = envVar
	}
	if envVar := os.Getenv("PASSWORD"); envVar != "" {
		config.Email.Password = envVar
	}
	if envVar := os.Getenv("MAILBOX"); envVar != "" {
		config.Email.Mailbox = envVar
	}
	if envVar := os.Getenv("WE_COM_URL"); envVar != "" {
		config.Webhook.WeComUrl = envVar
	}
	if envVar := os.Getenv("HOST"); envVar != "" {
		config.Llm.Host = envVar
	}
	if envVar := os.Getenv("API"); envVar != "" {
		config.Llm.Api = envVar
	}
	if envVar := os.Getenv("TOKEN"); envVar != "" {
		config.Llm.Token = envVar
	}
	if envVar := os.Getenv("MODEL"); envVar != "" {
		config.Llm.Model = envVar
	}
	if envVar := os.Getenv("PROMPT"); envVar != "" {
		config.Llm.Prompt = envVar
	}
	// 重复上述逻辑，为每个配置项覆盖环境变量
	// 确保为config.json文件中的每个字段检查环境变量
}

func main() {
	config := loadConfig("config.json")
	emailChan := make(chan *imap.Message)
	go processEmails(config, emailChan)
	go fetchEmails(config, emailChan)

	// 设置信号捕获，以便优雅地关闭程序
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan // 等待信号

	log.Println("Shutting down...")
	close(emailChan) // 关闭channel，通知其他goroutine停止工作
}

func fetchEmails(config Config, emailChan chan<- *imap.Message) {
	// 定义重连函数
	reconnect := func() *client.Client {
		for {
			c, err := client.DialTLS(config.Email.IMAPServer, nil)
			if err != nil {
				log.Printf("Reconnect DialTLS err: %v", err)
				time.Sleep(30 * time.Second) // 等待一段时间后重试
				continue
			}

			if err := c.Login(config.Email.Username, config.Email.Password); err != nil {
				log.Printf("Reconnect Login err: %v", err)
				c.Logout() // 确保关闭任何可能的连接
				time.Sleep(30 * time.Second)
				continue
			}

			log.Println("Reconnected and logged in.")
			return c // 成功重连并登录后返回新的客户端实例
		}
	}

	var c *client.Client
	log.Println("Connecting to server...")
	// 初始连接
	c = reconnect()
	log.Println("Logged in")
	// log.Println("Connecting to server...")

	for {
		_, err := c.Select("INBOX", false)
		if err != nil {
			if err.Error() == "imap: connection closed" {
				log.Println("Select INBOX err ", err)
				c = reconnect()
				continue
			}
			log.Println("Select INBOX err ", err)
			time.Sleep(3 * time.Second)
			continue
		}

		// 使用SEARCH命令找出所有未读的邮件
		criteria := imap.NewSearchCriteria()
		criteria.WithoutFlags = []string{imap.SeenFlag}
		ids, err := c.Search(criteria)
		if err != nil {
			log.Fatal("Search UNSEEN err ", err)
		}

		if len(ids) == 0 {
			log.Println("No new unread emails.")
			time.Sleep(1 * time.Minute)
			continue
		}

		// 只获取最新的未读邮件，最多3封
		var latestNCount uint32 = 3
		if uint32(len(ids)) > latestNCount {
			ids = ids[len(ids)-int(latestNCount):]
		}

		seqSet := new(imap.SeqSet)
		seqSet.AddNum(ids...)
		items := []imap.FetchItem{imap.FetchEnvelope, "BODY[TEXT]"}
		messages := make(chan *imap.Message, latestNCount)

		// 直接在当前goroutine中调用Fetch，避免在内部goroutine中关闭channel
		if err := c.Fetch(seqSet, items, messages); err != nil {
			log.Fatal("Fetch err ", err)
		}

		for msg := range messages {
			if msg.Envelope != nil {
				if strings.Contains(msg.Envelope.Subject, "的聊天记录") || strings.Contains(msg.Envelope.Subject, "Chat History") {
					fmt.Printf("Found email with subject: %s\n", msg.Envelope.Subject)
					emailChan <- msg
					// 标记邮件为已读
					seqSet := new(imap.SeqSet)
					seqSet.AddNum(msg.SeqNum)
					item := imap.FormatFlagsOp(imap.AddFlags, true)
					flags := []interface{}{imap.SeenFlag}
					if err := c.Store(seqSet, item, flags, nil); err != nil {
						log.Fatal("", err)
					}
				}
			}
		}
		// 等待一段时间再次检查
		time.Sleep(30 * time.Second)
	}
}

func processEmails(config Config, emailChan <-chan *imap.Message) {
	for msg := range emailChan {
		var emailBody string
		// 正确的使用 *imap.BodySectionName 作为键
		sectionName := imap.BodySectionName{}
		keys := make([]*imap.BodySectionName, 0, len(msg.Body))
		for bsn := range msg.Body {
			keys = append(keys, bsn)
		}
		for _, bsn := range keys {
			sectionName = *bsn
			if literal := msg.GetBody(&sectionName); literal != nil {
				bodyBytes, err := io.ReadAll(literal)
				if err != nil {
					log.Printf("Failed to read body: %v\n", err)
					continue
				}

				// 尝试解码Base64内容
				// 使用strings.TrimSpace移除可能的空白字符，包括换行符
				encodedContent := strings.TrimSpace(strings.Split(string(bodyBytes), "base64")[1])
				encodedContent = strings.TrimSpace(strings.Split(encodedContent, "--")[0])
				encodedContent = strings.ReplaceAll(encodedContent, "\n", "")
				encodedContent = strings.ReplaceAll(encodedContent, "\r", "")
				decodedBytes, err := base64.StdEncoding.DecodeString(encodedContent)
				if err != nil {
					log.Printf("Failed to decode base64 content: %v\n", err)
					// 如果解码失败，可能内容不是base64编码，可以选择直接使用原始内容
					emailBody = string(bodyBytes)
				} else {
					emailBody = string(decodedBytes)
				}
			} else {
				log.Println("No body found for message")
				continue
			}
		}

		prompt := strings.Replace(config.Llm.Prompt, "[CHAT-RECORD]", emailBody, -1)
		summaryContent, err := CallChatGPT(config, prompt, config.Llm.Token)
		if err != nil {
			log.Printf("Error calling ChatGPT: %v\n", err)
			continue // 在出错时跳过当前邮件
		}

		// 邮件发送人
		sender := msg.Envelope.Sender[0].Address()

		// 发送摘要到WeCom
		sendSummaryToWeCom(summaryContent, config.Webhook.WeComUrl)

		// 发送摘要回复邮件
		sendSummaryEmail("摘要："+msg.Envelope.Subject, summaryContent, config.Email.Username, sender, config.Email.Password)
	}
}

// sendSummaryEmail 和 sendSummaryToWeCom 函数保持不变
func sendSummaryEmail(title, content, fromEmail, sendTo, password string) {
	// SMTP服务器的地址，使用587端口
	smtpHost := "smtp.qq.com"
	smtpPort := "587"

	// 设置SMTP客户端的配置
	auth := smtp.PlainAuth("", fromEmail, password, smtpHost)

	// 构造邮件内容
	msg := []byte("From: " + fromEmail + "\r\n" +
		"To: " + sendTo + "\r\n" +
		"Subject: " + title + "\r\n" +
		"MIME-Version: 1.0;\r\n" +
		"Content-Type: text/plain; charset=\"UTF-8\";\r\n" +
		"Content-Transfer-Encoding: 7bit\r\n" +
		"\r\n" +
		content + "\r\n")

	// 拼接SMTP服务器地址和端口
	addr := smtpHost + ":" + smtpPort

	// 使用smtp包的SendMail函数发送邮件
	err := smtp.SendMail(addr, auth, fromEmail, []string{sendTo}, msg)
	if err != nil {
		log.Fatal("Failed to send email: ", err)
	}

	log.Println("Email sent successfully")
}

func sendSummaryToWeCom(content, webhookUrl string) {
	log.Println("Sending summary to WeCom...", content)
	// Simplified WeCom webhook message sending
	msgObj := WebhookMessage{
		MsgType: "text",
		Text: struct {
			Content string `json:"content"`
		}{
			Content: content,
		},
	}

	// msg := fmt.Sprintf(`{"msgtype": "text", "text": {"content": "%s"}}`, content)
	msg, _ := json.Marshal(msgObj)
	_, err := http.Post(webhookUrl, "application/json", bytes.NewBuffer(msg))
	if err != nil {
		log.Fatal("Connect webhook err ", err)
	}
}

type WebhookMessage struct {
	MsgType string `json:"msgtype"`
	Text    struct {
		Content string `json:"content"`
	} `json:"text"`
}

// chatgpt llm
type ChatGPT struct {
	Request  ChatGPTRequest
	Response ChatGPTResponse
}

type ChatGPTRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	User     string    `json:"user"`
}

// write a func handle struct ChatGPTRequest to json string
func (r ChatGPTRequest) ToJson() ([]byte, error) {
	return json.Marshal(r)
}

type ChatGPTResponse struct {
	Choices []struct {
		Messages Message `json:"message"`
	} `json:"choices"`
}

// qwen llm
type Qwen struct {
	Request  QwenRequest
	Response QwenResponse
}

type QwenRequest struct {
	Model string `json:"model"`
	Input struct {
		Messages []Message `json:"messages"`
	} `json:"input"`
	Parameters struct {
		ResultFormat string `json:"result_format"`
	} `json:"parameters"`
}

type QwenResponse struct {
	Output struct {
		Choices []struct {
			Messages Message `json:"message"`
		} `json:"choices"`
	} `json:"output"`
}

func (r QwenRequest) ToJson() ([]byte, error) {
	return json.Marshal(r)
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Requester interface {
	ToJson() ([]byte, error)
}

// 定义一个接口来抽象不同响应类型共有的方法
type ApiResponse interface {
	GetContent() (string, error)
}

// 为 ChatGPTResponse 实现 GetContent 方法
func (r ChatGPTResponse) GetContent() (string, error) {
	if len(r.Choices) > 0 {
		return r.Choices[0].Messages.Content, nil
	}
	return "", fmt.Errorf("no response from API")
}

// 为 QwenResponse 实现 GetContent 方法
func (r QwenResponse) GetContent() (string, error) {
	if len(r.Output.Choices) > 0 {
		return r.Output.Choices[0].Messages.Content, nil
	}
	return "", fmt.Errorf("no response from API")
}

var httpClient = &http.Client{
	Timeout: time.Second * 30, // 设置一个合理的超时时间
}

// CallChatGPT 调用ChatGPT API并返回生成的文本
func CallChatGPT(config Config, prompt string, token string) (string, error) {
	var r Requester

	if strings.HasPrefix(config.Llm.Model, "qwen") {
		r = QwenRequest{
			Model: config.Llm.Model,
			Input: struct {
				Messages []Message `json:"messages"`
			}{
				Messages: []Message{{Role: "user", Content: prompt}},
			},
			Parameters: struct {
				ResultFormat string `json:"result_format"`
			}{
				ResultFormat: "message",
			},
		}
	} else {
		r = ChatGPTRequest{
			Model:    config.Llm.Model,
			Messages: []Message{{Role: "user", Content: prompt}},
			User:     "",
		}
	}
	requestBody, err := r.ToJson()
	if err != nil {
		return "", fmt.Errorf("error marshalling request body: %v", err)
	}

	// 构建HTTP请求
	req, err := http.NewRequest("POST", config.Llm.Host+config.Llm.Api, bytes.NewBufferString(string(requestBody)))
	if err != nil {
		return "", fmt.Errorf("error creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	// 发送请求
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("error calling ChatGPT API: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading API response: %v", err)
	}

	return parseLLMResponse(body, config.Llm.Model)
}

func parseLLMResponse(body []byte, model string) (string, error) {
	var resp ApiResponse // 使用接口类型的变量

	// 根据 model 的不同选择不同的响应结构体
	if strings.HasPrefix(model, "qwen") {
		resp = QwenResponse{}
	} else {
		resp = ChatGPTResponse{}
	}

	// 反序列化 JSON 到对应的结构体中
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("error unmarshalling API response: %v", err)
	}

	// 调用接口的 GetContent 方法来获取内容
	return resp.GetContent()
}