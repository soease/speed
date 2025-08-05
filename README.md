# 网络速度测试应用

> 本项目部分代码由AI辅助编写

一个简单但功能齐全的网络速度测试应用，能够测量下载速度、上传速度和网络延迟，并以图表形式展示测试结果。

## 功能特点

- 测量下载速度、上传速度和网络延迟
- 实时图表展示测试结果趋势
- 分组显示统计信息（平均/最高/最低速度和延迟）
- 显示运营商、服务器名称和距离信息
- 数据持久化存储（SQLite数据库）
- 简洁美观的Web界面

## 技术栈

- **后端**：Go语言
- **前端**：HTML, CSS, JavaScript
- **数据可视化**：Chart.js
- **数据库**：SQLite
- **网络测试库**：speedtest-go

## 项目结构

```
├── go.mod              # Go模块定义
├── go.sum              # 依赖包列表
├── main.go             # 主程序入口
├── webserver.go        # Web服务器实现
├── speedtest_results.db # SQLite数据库文件
├── templates/          # HTML模板
│   └── index.html      # 主页面
└── README.md           # 项目说明
```

## 安装和运行

### 前提条件

- 已安装Go语言环境（1.16+）
- 网络连接正常

### 运行步骤

1. 克隆或下载项目代码

```bash
git clone https://github.com/soease/speed.git
cd speed
```

2. 安装依赖

```bash
go mod tidy
```

3. 启动应用

```bash
go run . -web
```

4. 在浏览器中打开

```
http://localhost:8081
```

## 使用说明

### Web界面使用
1. 点击"开始测速"按钮进行网络速度测试
2. 测试完成后，结果将自动更新到图表和统计区域
3. 点击"刷新数据"按钮可手动刷新显示最新数据
4. 统计信息区域按下载速度、上传速度、延迟信息和其他信息分组显示
5. 其他信息区域显示运营商、服务器名称和距离

### 命令行使用
除了Web界面，应用还支持通过命令行进行操作：

1. **无参数运行**：使用默认服务器进行测速
```bash
./speedtest.exe
```

2. **列出所有可用服务器**：显示前50个可用服务器列表
```bash
./speedtest.exe -servers
```

3. **指定服务器ID进行测速**：使用特定ID的服务器进行测速
```bash
./speedtest.exe -serverid <服务器ID>
```

4. **列出所有测试记录**：显示保存在数据库中的测试记录
```bash
./speedtest.exe -list
```

5. **启动自动测速**：设置间隔时间（分钟），定期进行测速(一般与-web合用)
```bash
./speedtest.exe -interval <分钟数>
```

## 命令行参数说明

| 参数 | 描述 | 示例 |
|------|------|------|
| `-web` | 启动Web服务器展示统计图表 | `./speedtest.exe -web` |
| `-port` | 指定Web服务器端口（默认8081） | `./speedtest.exe -web -port 8080` |
| `-list` | 列出所有测试记录 | `./speedtest.exe -list` |
| `-servers` | 列出所有可用服务器 | `./speedtest.exe -servers` |
| `-serverid` | 指定服务器ID进行测速 | `./speedtest.exe -serverid 59386` |
| `-interval` | 自动测速间隔（分钟），0表示不自动测试 | `./speedtest.exe -interval 30` |
| `-limit` | 趋势图显示的最大测速记录数（默认100） | `./speedtest.exe -web -limit 200` |

## 截图展示

![应用界面](screenshot.png)

## 许可证

本项目基于MIT许可证进行修改，主要条款如下：

1. 您可以免费使用、复制、修改、合并、发布、分发、 sublicense 和/或销售本软件的副本
2. 在软件的所有副本或重要部分中，必须保留原始的版权声明和许可信息
3. **使用前请发送邮件通知至 [scwy@qq.com](mailto:scwy@qq.com) **
4. 软件按"原样"提供，不提供任何保证

详情请见LICENSE文件。

## 贡献

欢迎提交问题和拉取请求，为项目提供改进和新功能。

## 联系方式

如有问题或建议，请联系 [scwy@qq.com](mailto:scwy@qq.com)

网站: [https://i.scwy.net](https://i.scwy.net)