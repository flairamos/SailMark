SailMark Linux
==============

这是一个使用 Go + Gio 编写的轻量 Markdown 编辑器。
当前 Linux 包提供 Ubuntu x86_64 和 Rocky Linux 9 x86_64 两个构建。

运行
----
解压后进入目录，执行：

  chmod +x SailMark
  ./SailMark

也可以在命令行直接打开文件：

  ./SailMark /path/to/file.md

需要 X11 或 Wayland 图形环境。首次启动会加载系统字体。

Ubuntu 依赖
-----------
如果系统没有预装运行库，可执行：

  sudo apt update
  sudo apt install libegl1 libwayland-client0 libwayland-cursor0 \
    libwayland-egl1 libxkbcommon0 libxkbcommon-x11-0 libx11-6 \
    libx11-xcb1 libxcursor1 libxfixes3 libxrender1 libxcb1 \
    libglvnd0 libffi8

Rocky Linux 9 依赖
------------------
如果系统没有预装运行库，可执行：

  sudo dnf install mesa-libEGL libwayland-client libwayland-cursor \
    libwayland-egl libxkbcommon libxkbcommon-x11 libX11 libX11-xcb \
    libXcursor libXfixes libXrender libxcb libglvnd libffi

说明
----
- 图片支持本地 PNG / JPEG / GIF，支持相对路径、绝对路径和 file:// 路径
- GIF 在编辑器内显示首帧，在浏览器中可播放完整动画
- 网络图片暂不加载，会显示提示
- 未签名的 Linux 可执行文件首次运行时可能需要 chmod +x
- 更完整的功能说明和界面截图见同目录 README.md
