package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

var wg sync.WaitGroup
var sem = make(chan struct{}, 10) // 初始化 goroutines 的数量为 10

func handleFile(operation, target, key string) {
	sem <- struct{}{}
	defer func() {
		<-sem
		wg.Done()
	}()

	switch operation {
	case "encrypt":
		if strings.HasSuffix(target, ".encrypt") {
			return
		}
		encryptedFile := target + ".encrypt"
		var cmd *exec.Cmd
		if key == "" {
			fmt.Println("未指定密钥")
			return
		} else {
			cmd = exec.Command("gpg", "--output", encryptedFile, "--encrypt", "--recipient", key, target)
		}
		err := cmd.Run()
		if err != nil {
			fmt.Println("Failed to encrypt:", err)
			return
		}
		fmt.Println("加密完成: ", encryptedFile)
		err = os.Remove(target)
		if err != nil {
			fmt.Println("原始文件删除失败: ", err)
			return
		}

	case "decrypt":
		if !strings.HasSuffix(target, ".encrypt") {
			return
		}
		decryptedFile := strings.TrimSuffix(target, ".encrypt")
		var cmd *exec.Cmd
		cmd = exec.Command("gpg", "--output", decryptedFile, "--decrypt", target)
		err := cmd.Run()
		if err != nil {
			fmt.Println("解密失败: ", err)
			return
		}
		fmt.Println("解密完成: ", decryptedFile)
		err = os.Remove(target)
		if err != nil {
			fmt.Println("加密文件删除失败: ", err)
			return
		}

	case "sign":
		if strings.HasSuffix(target, ".sig") {
			return
		}
		signatureFile := target + ".sig"
		var cmd *exec.Cmd
		if key == "" {
			cmd = exec.Command("gpg", "--detach-sig", "--output", signatureFile, "--sign", target)
		} else {
			cmd = exec.Command("gpg", "--detach-sig", "--output", signatureFile, "--local-user", key, "--sign", target)
		}
		err := cmd.Run()
		if err != nil {
			fmt.Println("签名失败: ", err)
			return
		}
		fmt.Println("签名完成: ", target, "-->", signatureFile)

	case "verify":
		signedFile := target
		originalFile := strings.TrimSuffix(target, ".sig")
		if !strings.HasSuffix(target, ".sig") {
			signedFile = target + ".sig"
		}
		var cmd *exec.Cmd
		cmd = exec.Command("gpg", "--verify", signedFile)
		err := cmd.Run()
		if err != nil {
			fmt.Println("验证签名失败", originalFile, "<-->", signedFile)
			return
		}
		fmt.Println("验证签名成功: ", originalFile, "<-->", signedFile)

	default:
		fmt.Println("未知操作: ", operation)
		fmt.Println("请使用 'encrypt', 'decrypt', 'sign' 或 'verify'")
		os.Exit(1)
	}
}

func completionScript(programName string) {
	script := `#compdef %s

local -a args

args+=(
    '-encrypt[加密选项]'
    '-decrypt[解密选项]'
    '-sign[签名选项]'
    '-verify[验证签名选项]'
    '-target[文件|目录]:*:file:_files'
    '-key[指定密钥]'
    '-completion-script[Zsh 自动补全脚本]'
)

_arguments -s $args
`
	fmt.Printf(script, programName)
}

func main() {
	programName := filepath.Base(os.Args[0])
	encrypt := flag.Bool("encrypt", false, "加密文件/目录")
	decrypt := flag.Bool("decrypt", false, "解密文件/目录")
	sign := flag.Bool("sign", false, "签名文件")
	verify := flag.Bool("verify", false, "验证签名")
	target := flag.String("target", "", "目标文件/目录")
	key := flag.String("key", "", "加密密钥")
	script := flag.Bool("completion-script", false, "Zsh 自动补全脚本")
	flag.Parse()

	if *script {
		completionScript(programName)
		os.Exit(0)
	}

	operateCount := 0

	if *encrypt {
		operateCount++
	}
	if *decrypt {
		operateCount++
	}
	if *sign {
		operateCount++
	}
	if *verify {
		operateCount++
	}

	if operateCount == 0 {
		fmt.Println("错误: 参数 -encrypt, -decrypt, -sign 或 -verify 缺失")
		os.Exit(1)
	}

	if operateCount > 1 {
		fmt.Println("错误: 参数 -encrypt, -decrypt, -sign 和 -verify 冲突")
		os.Exit(1)
	}

	info, err := os.Stat(*target)
	if err != nil {
		fmt.Println("获取目标信息失败: ", err)
		os.Exit(1)
	}

	operation := "encrypt"
	if *decrypt {
		operation = "decrypt"
	} else if *sign {
		operation = "sign"
	} else if *verify {
		operation = "verify"
	}

	if info.IsDir() {
		filepath.Walk(*target, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				fmt.Println("遍历目录错误: ", err)
				return err
			}
			if info.Mode().IsRegular() && info.Name() != ".DS_Store" {
				wg.Add(1)
				go handleFile(operation, path, *key)
			}
			return nil
		})
		wg.Wait()
	} else if info.Mode().IsRegular() {
		wg.Add(1)
		go handleFile(operation, *target, *key)
		wg.Wait()
	} else {
		fmt.Println("目标不是文件或者目录: ", *target)
		os.Exit(1)
	}
}
