SailMark
========

SailMark 是一个使用 Gio 编写的轻量 Markdown 编辑器，交互方式接近 Typora：
打开文件后展示渲染后的 Markdown；点击某一内容块，该块会切换为显示
Markdown 源码的编辑状态；点击空白处、切换其他内容块或按 Esc 后自动恢复渲染。

顶部菜单仅包含：
  打开文件     Ctrl+O
  保存文件     Ctrl+S
  关闭文件     Ctrl+W
  查找         Ctrl+F

查找栏中按 Enter 跳到下一个结果，点击“上一个/下一个”也可以切换结果。
F3 或 Ctrl+G 同样可以跳到下一个查找结果，按 Esc 关闭查找栏。

编辑内容块时：
  - 直接输入即可修改 Markdown 源码；
  - 点击空白处或另一段内容，当前内容自动渲染；
  - 按 Esc 或 Ctrl+Enter 完成编辑。

文件保存时会保留原文件使用的换行风格（LF 或 CRLF）。关闭有未保存更改的文件时，
SailMark 会询问是否保存。

运行环境
--------
Windows 10/11 x64。直接双击 SailMark.exe，无需安装。
程序会使用 Windows 自带的“微软雅黑”与 Consolas 字体。
支持本地 PNG / JPEG / GIF 图片；GIF 在编辑器内显示首帧。

从源码构建
----------
需要 Go 1.24 或更高版本。在项目目录执行：

  build_windows.bat

或手动执行：

  set GOOS=windows
  set GOARCH=amd64
  set CGO_ENABLED=0
  go build -trimpath -ldflags="-s -w -H=windowsgui" -o dist\SailMark\SailMark.exe .

项目结构
--------
  app.go      应用状态、事件、编辑与查找逻辑
  ui.go       菜单、查找栏、文档视口、状态栏与对话框
  render.go   Markdown 块渲染与可直接编辑的文本块
  markdown.go Markdown 解析、块划分与搜索纯文本
  fonts.go    系统字体加载
  dialog_*    原生文件选择对话框
