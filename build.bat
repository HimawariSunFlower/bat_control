chcp 65001
go.exe build -ldflags "-s -w -H=windowsgui" -o bat_control.exe ./main.go

echo "等待退出"
ping 1.1.1.1 -n 1 -w 3000
:pause