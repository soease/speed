package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/showwin/speedtest-go/speedtest"
)

// 全局静态变量，统一数据库路径
var DBPath string

func init() {
	// 获取当前可执行文件的目录
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)
	DBPath = filepath.Join(dir, "results.db")
}

// 统一打开数据库的函数
func openDatabase() (*sql.DB, error) {
	db, err := sql.Open("sqlite3", DBPath)
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %v", err)
	}

	// 验证连接
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("验证数据库连接失败: %v", err)
	}

	return db, nil
}

// 列出数据库中的所有测试结果
func listResults() {
	// 连接数据库
	db, err := openDatabase()
	if err != nil {
		log.Fatalf("%v", err)
	}
	defer db.Close()

	// 查询数据
	rows, err := db.Query("SELECT id, isp, server_name, server_country, server_distance, latency, download_speed, upload_speed, test_time FROM speedtest_results ORDER BY test_time DESC")
	if err != nil {
		log.Fatalf("查询数据失败: %v", err)
	}
	defer rows.Close()

	// 打印表头
	fmt.Printf("%-5s %-20s %-30s %-15s %-10s %-8s %-12s %-12s %-20s\n",
		"ID", "运营商", "服务器名称", "国家", "距离(km)", "延迟(ms)", "下载速度(Mbps)", "上传速度(Mbps)", "测试时间")
	fmt.Println("--------------------------------------------------------------------------------------------------------------------------------------------------------------------")

	// 遍历结果
	for rows.Next() {
		var id int
		var isp, serverName, serverCountry, testTime string
		var serverDistance, downloadSpeed, uploadSpeed float64
		var latency int

		err := rows.Scan(&id, &isp, &serverName, &serverCountry, &serverDistance, &latency, &downloadSpeed, &uploadSpeed, &testTime)
		if err != nil {
			log.Fatalf("扫描数据失败: %v", err)
		}

		// 打印一行结果
		fmt.Printf("%-5d %-20s %-30s %-15s %-10.2f %-8d %-12.2f %-12.2f %-20s\n",
			id, isp, serverName, serverCountry, serverDistance, latency, downloadSpeed, uploadSpeed, testTime)
	}

	if err = rows.Err(); err != nil {
		log.Fatalf("遍历结果失败: %v", err)
	}
}

// 自动测速函数
func autoTest(interval int) {
	ticker := time.NewTicker(time.Duration(interval) * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// 执行测速
			user, err := speedtest.FetchUserInfo()
			if err != nil {
				log.Printf("获取用户信息失败: %v", err)
				continue
			}

			// 获取全球Speedtest服务器列表
			servers, err := speedtest.FetchServers()
			if err != nil {
				log.Printf("获取服务器列表失败: %v", err)
				continue
			}

			// 筛选最优服务器
			targets, err := servers.FindServer([]int{})
			if err != nil {
				log.Printf("筛选服务器失败: %v", err)
				continue
			}
			server := targets[0]

			// 测试延迟
			server.PingTest(func(latency time.Duration) {})

			// 测试下载速度
			server.DownloadTest()
			downloadMbps := float64(server.DLSpeed) * 8 / 1e6

			// 测试上传速度
			server.UploadTest()
			uploadMbps := float64(server.ULSpeed) * 8 / 1e6

			// 保存测试结果到数据库
			db, err := openDatabase()
			if err != nil {
				log.Printf("%v", err)
				continue
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
				continue
			}

			log.Printf("自动测速完成: 下载 %.2f Mbps, 上传 %.2f Mbps, 延迟 %d ms", downloadMbps, uploadMbps, server.Latency.Milliseconds())
		}
	}
}

func main() {
	// 解析命令行参数
	listFlag := flag.Bool("list", false, "列出所有测试记录")
	webFlag := flag.Bool("web", false, "启动Web服务器展示统计图表")
	portFlag := flag.String("port", "8080", "Web服务器端口")
	intervalFlag := flag.Int("interval", 0, "自动测速间隔(分钟)，0表示不自动测试")
	limitFlag := flag.Int("limit", 100, "趋势图显示的最大测速记录数，默认100")
	serverListFlag := flag.Bool("servers", false, "列出所有可用服务器")
	serverIDFlag := flag.String("serverid", "", "指定服务器ID进行测速")
	flag.Parse()

	// 当启用Web服务器且未指定测速间隔时，默认设置为120分钟(2小时)
	if *webFlag && *intervalFlag == 0 {
		*intervalFlag = 120
	}

	// 如果指定了-list参数，则列出记录并退出
	if *listFlag {
		listResults()
		return
	}

	// 如果指定了-servers参数，则列出所有可用服务器并退出
	if *serverListFlag {
		// 获取用户信息
		user, err := speedtest.FetchUserInfo()
		if err != nil {
			log.Fatalf("获取用户信息失败: %v", err)
		}

		// 获取公网IP的经纬度信息
		ip := user.IP
		var ipLat, ipLon string
		resp, err := http.Get(fmt.Sprintf("http://ip-api.com/json/%s?lang=zh-CN", ip))
		if err == nil {
			defer resp.Body.Close()
			var result map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&result); err == nil {
				if result["status"] == "success" {
					ipLat = fmt.Sprintf("%v", result["lat"])
					ipLon = fmt.Sprintf("%v", result["lon"])
				}
			}
		}

		fmt.Printf("您的运营商: %s, 公网IP: %s, 经纬度: %s, %s\n\n", user.Isp, user.IP, ipLat, ipLon)

		// 获取全球Speedtest服务器列表
		servers, err := speedtest.FetchServers()
		if err != nil {
			log.Fatalf("获取服务器列表失败: %v", err)
		}

		// 打印表头
		fmt.Printf("%-10s %-30s %-15s %-10s %-10s\n",
			"服务器ID", "服务器名称", "国家", "距离(km)", "延迟(ms)")
		fmt.Println("---------------------------------------------------------------------")

		// 遍历服务器列表（只显示前50个，避免输出过多）
		count := 0
		for _, s := range servers {
			if count >= 50 {
				break
			}
			// 进行延迟测试
			serverCopy := s
			serverCopy.PingTest(nil) // 使用nil回调，仅执行ping测试

			// 获取延迟信息
			latency := serverCopy.Latency.Milliseconds()

			// 格式化输出，如果延迟为0则显示为超时
			var latencyStr string
			if latency > 0 {
				latencyStr = fmt.Sprintf("%d", latency)
			} else {
				latencyStr = "超时"
			}

			fmt.Printf("%-10s %-30s %-15s %-10.2f %-10s\n",
				s.ID, s.Name, s.Country, s.Distance, latencyStr)
			count++
		}

		if len(servers) > 50 {
			fmt.Printf("\n只显示前50个服务器，共%d个服务器可用\n", len(servers))
		}
		return
	}

	// 如果指定了-web参数，则启动Web服务器
	if *webFlag {
		// 如果同时指定了interval参数且大于0，则启动自动测速
		if *intervalFlag > 0 {
			go autoTest(*intervalFlag)
			log.Printf("已启动Web服务器和自动测速，间隔为%d分钟\n", *intervalFlag)
		} else {
			log.Println("已启动Web服务器")
		}
		startWebServer(*portFlag, *limitFlag)
		return
	}

	// 如果指定了interval参数且大于0，则启动自动测速
	if *intervalFlag > 0 {
		go autoTest(*intervalFlag)
		fmt.Printf("已启动自动测速，间隔为%d分钟\n", *intervalFlag)
	}

	// 如果既没有指定-web也没有指定自动测速，则执行一次测速然后退出
	if !*webFlag && *intervalFlag == 0 {
		// 1. 初始化客户端并获取服务器信息
		user, err := speedtest.FetchUserInfo()
		if err != nil {
			log.Fatalf("获取用户信息失败: %v", err)
		}
		fmt.Printf("运营商: %s\n", user.Isp)

		// 获取全球Speedtest服务器列表
		servers, err := speedtest.FetchServers()
		if err != nil {
			log.Fatalf("获取服务器列表失败: %v", err)
		}

		// 2. 选择服务器
		var server *speedtest.Server
		if *serverIDFlag != "" {
			// 如果指定了服务器ID，则使用该服务器
			serverID, err := strconv.Atoi(*serverIDFlag)
			if err != nil {
				log.Fatalf("无效的服务器ID: %v", err)
			}

			// 查找指定ID的服务器
			found := false
			for _, s := range servers {
				if s.ID == strconv.Itoa(serverID) {
					server = s
					found = true
					break
				}
			}

			if !found {
				log.Fatalf("未找到ID为%d的服务器", serverID)
			}

			// 测试该服务器的延迟
			server.PingTest(func(latency time.Duration) {})
			fmt.Printf("已选择服务器: %s (%s), 距离: %.2f km, 延迟: %d ms\n",
				server.Name, server.Country, server.Distance, server.Latency.Milliseconds())
		} else {
			// 否则，自动选择最近的服务器
			targets, err := servers.FindServer([]int{}) // 空参数表示自动筛选
			if err != nil {
				log.Fatalf("筛选服务器失败: %v", err)
			}
			server = targets[0] // 选择第一个（最近的）服务器
			// 测试延迟
			server.PingTest(func(latency time.Duration) {})
			fmt.Printf("自动选择服务器: %s (%s), 距离: %.2f km, 延迟: %d ms\n",
				server.Name, server.Country, server.Distance, server.Latency.Milliseconds())
		}

		// 3. 测试下载速度
		server.DownloadTest()
		// 转换单位：字节/秒 -> Mbps（1 B/s = 8 bit/s，1 Mbps = 1e6 bit/s）
		downloadMbps := float64(server.DLSpeed) * 8 / 1e6
		fmt.Printf("下载速度: %.2f Mbps\t", downloadMbps)

		// 4. 测试上传速度
		server.UploadTest()
		uploadMbps := float64(server.ULSpeed) * 8 / 1e6
		fmt.Printf("上传速度: %.2f Mbps\n", uploadMbps)

		// 5. 保存测试结果到SQLite数据库
		// 连接数据库
		db, err := sql.Open("sqlite3", DBPath)
		if err != nil {
			log.Fatalf("打开数据库失败: %v", err)
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
		test_time TIMESTAMP
	);
	`
		_, err = db.Exec(createTableSQL)
		if err != nil {
			log.Fatalf("创建表失败: %v", err)
		}

		// 插入数据
		testTime := time.Now().Format("2006-01-02 15:04:05")
		insertSQL := `
	INSERT INTO speedtest_results (isp, server_name, server_country, server_distance, latency, download_speed, upload_speed, test_time)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
		_, err = db.Exec(insertSQL, user.Isp, server.Name, server.Country, server.Distance, server.Latency.Milliseconds(), downloadMbps, uploadMbps, testTime)
		if err != nil {
			log.Fatalf("插入数据失败: %v", err)
		}
	}
}
