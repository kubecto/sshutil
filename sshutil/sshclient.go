package sshutil

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

// SSHClient 表示一个SSH客户端连接
type SSHClient struct {
	client *ssh.Client   // SSH连接客户端
	config *ssh.ClientConfig // SSH连接客户端配置
}

// NewSSHClient 创建一个新的SSHClient对象
func NewSSHClient(host string, port int, user string, password string) (*SSHClient, error) {
	sshConfig := &ssh.ClientConfig{
		User: user, // 连接用户名
		Auth: []ssh.AuthMethod{
			ssh.Password(password), // 连接密码
		},
		Timeout:         5 * time.Second, // 连接超时时间
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // 忽略主机公钥验证
	}

	// 通过TCP协议连接远程主机
	client, err := ssh.Dial("tcp", net.JoinHostPort(host, strconv.Itoa(port)), sshConfig)
	if err != nil {
		return nil, err
	}

	return &SSHClient{client: client, config: sshConfig}, nil
}

// Close 关闭SSHClient连接
func (c *SSHClient) Close() error {
	return c.client.Close()
}

// RunCommand 执行远程命令
func (c *SSHClient) RunCommand(command string) (string, error) {
	session, err := c.client.NewSession() // 创建新的SSH会话
	if err != nil {
		return "", err
	}
	defer session.Close() // 确保会话结束后关闭

	output, err := session.Output(command) // 执行命令并获取输出结果
	if err != nil {
		return "", err
	}

	return string(output), nil
}

// CopyFile 传输本地文件到远程主机
func (c *SSHClient) CopyFile(localPath string, remotePath string) error {
	src, err := os.Open(localPath) // 打开本地文件
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := c.client.Create(remotePath) // 在远程主机上创建文件
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(dst, src) // 将本地文件内容拷贝到远程文件中
	if err != nil {
		return err
	}

	return nil
}

// CopyDir 将本地目录复制到远程服务器
func (c *SSHClient) CopyDir(localPath string, remotePath string) error {
	// 遍历本地目录树
	err := filepath.Walk(localPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 获取目标路径
		relPath, err := filepath.Rel(localPath, path)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(remotePath, relPath)

		if info.IsDir() {
			// 如果远端目录不存在，请创建远端目录
			err = c.RunCommand(fmt.Sprintf("mkdir -p '%s'", dstPath))
			if err != nil {
				return err
			}
		} else {
			// 将文件复制到远程服务器
			wg := &sync.WaitGroup{}
			wg.Add(1)
			go func() {
				defer wg.Done()
				err = c.CopyFile(path, dstPath)
			}()
			wg.Wait()
			if err != nil {
				return err
			}
		}

		return nil
	})

	return err
}

