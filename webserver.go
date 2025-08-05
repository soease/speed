package main

import (
	"database/sql"
	"encoding/json"
	"embed"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/showwin/speedtest-go/speedtest"
)

//go:embed templates/*
var templatesFS embed.FS

// 反转字符串切片
func reverseStringSlice(s []string) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}

// 反转float64切片
func reverseFloat64Slice(s []float64) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}

// 反转int切片
func reverseIntSlice(s []int) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}

// GetIndexTemplate 从嵌入式文件系统加载并解析index.html模板
func GetIndexTemplate() (*template.Template, error) {
	return template.ParseFS(templatesFS, "templates/index.html")
}

// 定义用于存储图表数据的结构
type SpeedData struct {
	TestTime      string  `json:"test_time"`
	DownloadSpeed float64 `json:"download_speed"`
	UploadSpeed   float64 `json:"upload_speed"`
	Latency       int     `json:"latency"`
}

// 定义测试结果结构
type TestResult struct {
	DownloadSpeed float64 `json:"download_speed"`
	UploadSpeed   float64 `json:"upload_speed"`
	Latency       int     `json:"latency"`
	ISP           string  `json:"isp"`
	ServerName    string  `json:"server_name"`
}

// 执行测速处理函数
func runTestHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 1. 初始化客户端并获取服务器信息
	user, err := speedtest.FetchUserInfo()
	if err != nil {
		log.Printf("获取用户信息失败: %v", err)
		http.Error(w, "获取用户信息失败", http.StatusInternalServerError)
		return
	}

	// 获取全球Speedtest服务器列表
	servers, err := speedtest.FetchServers()
	if err != nil {
		log.Printf("获取服务器列表失败: %v", err)
		http.Error(w, "获取服务器列表失败", http.StatusInternalServerError)
		return
	}

	// 2. 筛选最优服务器
	targets, err := servers.FindServer([]int{})
	if err != nil {
		log.Printf("筛选服务器失败: %v", err)
		http.Error(w, "筛选服务器失败", http.StatusInternalServerError)
		return
	}
	server := targets[0]

	// 3. 测试延迟
	server.PingTest(func(latency time.Duration) {
		// 回调函数为空
	})

	// 4. 测试下载速度
	server.DownloadTest()
	downloadMbps := float64(server.DLSpeed) * 8 / 1e6

	// 5. 测试上传速度
	server.UploadTest()
	uploadMbps := float64(server.ULSpeed) * 8 / 1e6

	// 6. 保存测试结果到数据库
	db, err := sql.Open("sqlite3", "./speedtest_results.db")
	if err != nil {
		log.Printf("打开数据库失败: %v", err)
		http.Error(w, "保存结果失败", http.StatusInternalServerError)
		return
	}
	defer db.Close()

	// 插入数据
	testTime := time.Now().Format("2006-01-02 15:04:05")
	insertSQL := `
	INSERT INTO speedtest_results (isp, server_name, server_country, server_distance, latency, download_speed, upload_speed, test_time)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err = db.Exec(insertSQL, user.Isp, server.Name, server.Country, server.Distance, server.Latency.Milliseconds(), downloadMbps, uploadMbps, testTime)
	if err != nil {
		log.Printf("插入数据失败: %v", err)
		http.Error(w, "保存结果失败", http.StatusInternalServerError)
		return
	}

	// 7. 返回测试结果
	result := TestResult{
		DownloadSpeed: downloadMbps,
		UploadSpeed:   uploadMbps,
		Latency:       int(server.Latency.Milliseconds()),
		ISP:           user.Isp,
		ServerName:    server.Name,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// 首页处理函数
func indexHandler(w http.ResponseWriter, r *http.Request) {
	tmpl, err := GetIndexTemplate()
	if err != nil {
		log.Fatalf("解析模板失败: %v", err)
	}
	tmpl.Execute(w, nil)
}

// 获取图表数据的API，接受limit参数限制返回的记录数
func chartDataHandler(limit int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 连接数据库
		db, err := sql.Open("sqlite3", "./speedtest_results.db")
		if err != nil {
			log.Printf("打开数据库失败: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		defer db.Close()

		// 查询数据
		// 使用strftime函数确保时间格式为'MM-DD HH:MM'
		rows, err := db.Query("SELECT strftime('%m-%d %H:%M', test_time) as test_time, download_speed, upload_speed, latency FROM speedtest_results ORDER BY test_time DESC LIMIT ?", limit)
		if err != nil {
			log.Printf("查询数据失败: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		// 分离查询结果为三个独立的数组
		var downloadData []float64
		var uploadData []float64
		var latencyData []int
		var labels []string

		for rows.Next() {
			var testTime string
			var downloadSpeed, uploadSpeed float64
			var latency int

			err := rows.Scan(&testTime, &downloadSpeed, &uploadSpeed, &latency)
			if err != nil {
				log.Printf("扫描数据失败: %v", err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}

			labels = append(labels, testTime)
			downloadData = append(downloadData, downloadSpeed)
			uploadData = append(uploadData, uploadSpeed)
			latencyData = append(latencyData, latency)
		}

		// 返回JSON数据
		w.Header().Set("Content-Type", "application/json")
		// 反转数据，确保时间顺序从旧到新
		reverseStringSlice(labels)
		reverseFloat64Slice(downloadData)
		reverseFloat64Slice(uploadData)
		reverseIntSlice(latencyData)

		// 获取最近一次测试的运营商、服务器名称和距离信息
		var isp, serverName string
		var distance float64

		// 查询最新的一条记录
		err = db.QueryRow("SELECT isp, server_name, server_distance FROM speedtest_results ORDER BY test_time DESC LIMIT 1").Scan(&isp, &serverName, &distance)
		if err != nil && err != sql.ErrNoRows {
			log.Printf("查询服务器信息失败: %v", err)
			// 不中断程序，继续返回其他数据
		}

		// 返回JSON数据
		json.NewEncoder(w).Encode(map[string]interface{}{
			"labels":       labels,
			"downloadData": downloadData,
			"uploadData":   uploadData,
			"latencyData":  latencyData,
			"isp":          isp,
			"serverName":   serverName,
			"distance":     distance,
		})
	}
}

// 初始化数据库
func initDatabase() error {
	db, err := sql.Open("sqlite3", "./speedtest_results.db")
	if err != nil {
		return fmt.Errorf("打开数据库失败: %v", err)
	}
	defer db.Close()

	// 创建表
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS speedtest_results (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		isp TEXT,
		server_name TEXT,
		server_country TEXT,
		server_distance REAL,
		latency INTEGER,
		download_speed REAL,
		upload_speed REAL,
		test_time TEXT
	)
	`
	_, err = db.Exec(createTableSQL)
	if err != nil {
		return fmt.Errorf("创建表失败: %v", err)
	}

	return nil
}

// 启动Web服务器
func startWebServer(port string, limit int) {
	// 初始化数据库
	if err := initDatabase(); err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}

	// 创建templates目录
	// 注意：在实际运行前需要手动创建templates目录并放置index.html文件

	// 注册处理函数
	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/api/chart-data", chartDataHandler(limit))
	http.HandleFunc("/api/run-test", runTestHandler)

	// 启动服务器
	log.Printf("Web服务器已启动，监听端口: %s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
