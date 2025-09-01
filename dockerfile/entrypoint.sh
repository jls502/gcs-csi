#!/bin/sh

# 启动 crond
echo "Starting synctoken ..."
/usr/local/bin/synctoken &

# 检查 crond 是否运行
if pgrep synctoken > /dev/null; then
    echo "synctoken started successfully"
else
    echo "Failed to start synctoken" >&2
    exit 1
fi

# 执行主应用
echo "Starting main application..."
exec "$@"
