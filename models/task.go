package models

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/beego/beego/v2/client/httplib"
	"github.com/beego/beego/v2/core/logs"
)

type Task struct {
	ID      int
	Cron    string
	Path    string
	Enable  bool
	Mode    string //obo alo
	Word    string
	Run     func()
	Name    string
	Timeout int
	Envs    []Env
	Args    string
	Ykq     bool
	Hack    bool
}

type Env struct {
	Name  string
	Value string
}

func initTask() {
	for i := range Config.Tasks {
		if Config.Tasks[i].Cron != "" {
			createTask(&Config.Tasks[i])
		}
	}
}

func createTask(task *Task) {
	id, err := c.AddFunc(task.Cron, func() {
		runTask(task)
	})
	if err != nil {
		logs.Warn(task.Word, "任务创建失败")
	} else {
		task.ID = int(id)
		logs.Info(task.Word, "任务创建成功")
	}
}

func runTask(task *Task, msgs ...interface{}) string {
	msg := ""
	if task.Name == "" {
		slice := strings.Split(task.Path, "/")
		len := len(slice)
		if len == 0 {
			logs.Warn("取法识别的文件名")
			return ""
		}
		task.Name = slice[len-1]
	}
	var path = ExecPath + "/scripts/" + task.Name
	if strings.Contains(task.Path, "http") {
		f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0777)
		if err != nil {
			logs.Warn("打开%s失败，", path, err)
			return ""
		}
		url := task.Path
		if strings.Contains(url, "raw.githubusercontent.com") {
			url = GhProxy + url
		}
		r, err := httplib.Get(url).Response()
		if err != nil {
			logs.Warn("下载%s失败，", task.Path, err)
		}
		io.Copy(f, r.Body)
		f.Close()
	} else {
		if path != task.Path && task.Name != task.Path {
			f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0777)
			if err != nil {
				logs.Warn("打开%s失败，", path, err)
				return ""
			}
			f2, err := os.Open(task.Path)
			if err != nil {
				f.Close()
				logs.Warn("打开%s失败，", path, err)
				return ""
			}
			io.Copy(f, f2)
			f2.Close()
			f.Close()
		}
	}
	lan := Config.Node
	if strings.Contains(task.Name, ".py") {
		lan = Config.Python
	}
	envs := ""
	for _, env := range task.Envs {
		envs += fmt.Sprintf("export %s=\"%s\"", env.Name, env.Value)
	}
	sh := fmt.Sprintf(`
%s
%s %s
	`, envs,
		lan, task.Name)
	cmd := exec.Command("sh", "-c", sh)
	stdout, err := cmd.StdoutPipe()
	stderr, err := cmd.StderrPipe()
	if err != nil {
		logs.Warn("cmd.StdoutPipe: ", err)
		return ""
	}
	cmd.Dir = ExecPath + "/scripts/"
	err = cmd.Start()
	if err != nil {
		logs.Warn("%v", err)
		return ""
	}
	go func() {
		msg := ""
		reader := bufio.NewReader(stderr)
		for {
			line, err2 := reader.ReadString('\n')
			if err2 != nil || io.EOF == err2 {
				break
			}
			msg += line
		}
		if msg != "" {
			sendMessagee(msg, msgs...)
		}
	}()
	reader := bufio.NewReader(stdout)
	for {
		line, err2 := reader.ReadString('\n')
		if err2 != nil || io.EOF == err2 {
			break
		}
		msg += line
		if !task.Ykq && len(msgs) > 0 {
			sendMessagee(strings.Replace(line, "\n", "", -1), msgs...)
		}
	}
	if task.Ykq && len(msgs) > 0 {
		sendMessagee(msg, msgs...)
	}
	err = cmd.Wait()
	return msg
}

func cmd(str string, msgs ...interface{}) {
	cmd := exec.Command("sh", "-c", str)
	stdout, err := cmd.StdoutPipe()
	stderr, err := cmd.StderrPipe()
	if err != nil {
		logs.Warn("cmd.StdoutPipe: ", err)
		return
	}
	cmd.Dir = ExecPath + "/scripts/"
	err = cmd.Start()
	if err != nil {
		logs.Warn("%v", err)
		return
	}
	go func() {
		msg := ""
		reader := bufio.NewReader(stderr)
		for {
			line, err2 := reader.ReadString('\n')
			if err2 != nil || io.EOF == err2 {
				break
			}
			msg += line
		}
		if msg != "" {
			sendMessagee(msg, msgs...)
		}
	}()
	msg := ""
	reader := bufio.NewReader(stdout)
	st := time.Now()
	for {
		line, err2 := reader.ReadString('\n')
		if err2 != nil || io.EOF == err2 {
			break
		}
		msg += line
		nt := time.Now()
		if (nt.Unix() - st.Unix()) > 1 {
			go sendMessagee(msg, msgs...)
			st = nt
			msg = ""
		}
	}
	if msg != "" {
		sendMessagee(msg, msgs...)
	}
	err = cmd.Wait()
}
