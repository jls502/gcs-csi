package main

import (
	"io"
	"log"
	"os"

	"github.com/fsnotify/fsnotify"
)

func main() {
	srcFile := "/var/run/service-account/token"
	dstFile := "/host/var/run/service-account/token"

	// 初始同步
	if err := copyFile(srcFile, dstFile); err != nil {
		log.Printf("Initial sync failed: %v", err)
	}

	// 设置文件监控
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatalf("Watcher creation failed: %v", err)
	}
	defer watcher.Close()

	if err := watcher.Add(srcFile); err != nil {
                log.Fatalf("Failed to watch token file: %v", err)
        }

	// 处理事件
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			log.Printf("event:%v, modified file:%v",event, event.Name)

			if event.Name == srcFile && (event.Op&fsnotify.Write == fsnotify.Write ||
                 				event.Op&fsnotify.Create == fsnotify.Create ||
                 				event.Op&fsnotify.Rename == fsnotify.Rename) {
                		log.Printf("Detected change in %s, syncing to %s", srcFile, dstFile)
				if err := copyFile(srcFile, dstFile); err != nil {
                			log.Printf("sync file failed: %v", err)
        			} else {
					log.Println("sync file success")
				}
                		// 如果是重命名，重新添加 watcher
                		if event.Op&fsnotify.Rename == fsnotify.Rename {
                    			watcher.Remove(srcFile)
                    			_ = watcher.Add(srcFile)
                		}
            		}
		case err, ok := <-watcher.Errors:
			if !ok {
				log.Printf("Watcher no error")
				return
			}
			log.Printf("Watcher error: %v", err)
		}
	}
	//}()
}

// 文件拷贝函数
func copyFile(src, dst string) error {
    in, err := os.Open(src)
    if err != nil {
        return err
    }
    defer in.Close()
    out, err := os.Create(dst)
    if err != nil {
        return err
    }
    defer out.Close()
    _, err = io.Copy(out, in)
    return err
}
