# ====================== 项目基础配置 ======================
# 项目名称
APP_NAME := go-snake-game
# 编译后二进制文件输出目录
OUTPUT_DIR := bin
# 项目包含的三个独立服务：网关、登录、贪吃蛇逻辑服
SERVICES := gateway login game

# ====================== 自动区分 Windows / Mac/Linux 系统 ======================
# 判断当前操作系统
ifeq ($(OS),Windows_NT)
	# Windows 系统：删除文件夹命令
	RM = rmdir /s /q
	# Windows 可执行文件后缀 .exe
	EXE_SUFFIX = .exe
	# Windows 创建文件夹命令
	MKDIR = mkdir
else
	# Mac / Linux 系统：递归强制删除文件夹
	RM = rm -rf
	# Mac/Linux 无exe后缀，空字符串
	EXE_SUFFIX =
	# Mac/Linux 创建文件夹，不存在则创建，存在不报错
	MKDIR = mkdir -p
endif

# ====================== 声明伪目标（不是文件，只是命令） ======================
# 避免和项目内同名文件冲突，必须写
.PHONY: all build build-svc fmt lint tidy test clean help

# 默认执行命令：直接输入 make 等价于 make build
all: build

# ====================== 1. 编译全部三个服务 ======================
build:
	# 创建输出bin目录，不存在则新建
	@$(MKDIR) $(OUTPUT_DIR)
	# 循环遍历 gateway login game，逐个编译
	@for svc in $(SERVICES); do \
		echo "编译 $$svc ..."; \
		# go build 编译对应服务，输出到bin目录，自动带系统exe后缀
		go build -o $(OUTPUT_DIR)/$$svc$(EXE_SUFFIX) cmd/$$svc/main.go; \
	done
	@echo "✅ 全部服务编译完成，输出至 bin/"

# ====================== 2. 单独编译某一个服务 ======================
# 使用示例：make build-svc SERVICE=game
build-svc:
	@$(MKDIR) $(OUTPUT_DIR)
	go build -o $(OUTPUT_DIR)/$(SERVICE)$(EXE_SUFFIX) cmd/$(SERVICE)/main.go
	@echo "✅ $(SERVICE) 编译完成"

# ====================== 3. 格式化所有Go代码 ======================
# 统一代码缩进、换行、语法排版，符合Go官方规范
fmt:
	go fmt ./...

# ====================== 4. 代码静态检查（规范、潜在bug检测） ======================
# 检查未使用变量、错误规范、内存风险、并发隐患等问题
lint:
	golangci-lint run ./...

# ====================== 5. 整理依赖 ======================
# 自动清理无用依赖、补全缺失依赖，更新go.sum文件
tidy:
	go mod tidy

# ====================== 6. 运行全部单元测试 ======================
# -v 打印详细执行日志，后续写单测后使用
test:
	go test -v ./...

# ====================== 7. 清理编译产物 ======================
# 删除整个bin文件夹，清空编译出的二进制程序
clean:
	$(RM) $(OUTPUT_DIR)

# ====================== 8. 查看所有可用make命令帮助 ======================
# 输入 make help 查看全部指令说明
help:
	@echo "可用命令："
	@echo "  make build      编译当前系统全部3个服务"
	@echo "  make build-svc SERVICE=gateway 单独编译指定服务"
	@echo "  make fmt        自动格式化全部Go代码"
	@echo "  make lint       代码规范&风险静态检测"
	@echo "  make tidy       自动整理/更新项目依赖"
	@echo "  make test       执行项目所有单元测试"
	@echo "  make clean      删除bin目录下所有编译文件"
