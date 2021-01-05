package flexbot

import (
	"fmt"
	"net"
	"bytes"
	"time"
	"regexp"
	"strconv"
	"golang.org/x/crypto/ssh"
)

func checkSshListen(host string) (listen bool) {
	timeout := time.Second
	conn, err := net.DialTimeout("tcp", host+":22", timeout)
	if err != nil {
		listen = false
	} else {
		listen = true
		conn.Close()
	}
	return
}

func checkSshCommand(host string, sshUser string, sshPrivateKey string) (err error) {
	var signer ssh.Signer
	var conn *ssh.Client
	var sess *ssh.Session
	if signer, err = ssh.ParsePrivateKey([]byte(sshPrivateKey)); err != nil {
		return
	}
	config := &ssh.ClientConfig {
		User: sshUser,
		Auth: []ssh.AuthMethod {
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	if conn, err = ssh.Dial("tcp", host + ":22", config); err != nil {
		return
	}
	defer conn.Close()
	if sess, err = conn.NewSession(); err != nil {
		return
	}
	err = sess.Run("uname -a")
	sess.Close()
	return
}

func runSshCommand(sshHost string, sshUser string, sshPrivateKey string, command string) (commandOutput string, err error) {
	var signer ssh.Signer
	var conn *ssh.Client
	var sess *ssh.Session
	var b_stdout, b_stderr bytes.Buffer
	if signer, err = ssh.ParsePrivateKey([]byte(sshPrivateKey)); err != nil {
		err = fmt.Errorf("runSshCommand(): failed to parse SSH private key: %s", err)
		return
	}
	config := &ssh.ClientConfig {
		User: sshUser,
		Auth: []ssh.AuthMethod {
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	if conn, err = ssh.Dial("tcp", sshHost + ":22", config); err != nil {
		err = fmt.Errorf("runSshCommand(): failed to connect to host %s: %s", sshHost, err)
		return
	}
	defer conn.Close()
	if sess, err = conn.NewSession(); err != nil {
		err = fmt.Errorf("runSshCommand(): failed to create SSH session: %s", err)
		return
	}
	defer sess.Close()
	sess.Stdout = &b_stdout
	sess.Stderr = &b_stderr
	err = sess.Run(command)
	if err != nil {
		err = fmt.Errorf("runSshCommand(): failed to run command %s: %s: %s", command, err, b_stderr.String())
		return
	}
	if b_stdout.Len() > 0 {
		commandOutput = b_stdout.String()
	}
	return
}

func stringSliceIntersection(src1, src2 []string) (dst []string) {
	hash := make(map[string]bool)
	for _, e := range src1 {
		hash[e] = true
	}
	for _, e := range src2 {
		if hash[e] {
			dst = append(dst, e)
		}
	}
	return
}

func stringSliceElementExists(array []string, elem string) (bool) {
	for _, e := range array {
		if e == elem {
			return true
		}
	}
	return false
}

func valueInRange(rangeStr string, value int) (bool, error) {
	var err error
	var rangeLower, rangeUpper int
	re := regexp.MustCompile(`([0-9]+)\s*-\s*([0-9]*)`)
	subMatch := re.FindStringSubmatch(rangeStr)
	if len(subMatch) == 3 {
		if rangeLower, err = strconv.Atoi(subMatch[1]); err != nil {
			return false, err
		}
		if rangeUpper, err = strconv.Atoi(subMatch[2]); err != nil {
			return false, err
		}
		return (value >= rangeLower && value <= rangeUpper), nil
	}
	if rangeLower, err =strconv.Atoi(rangeStr); err != nil {
		return false, err
	}
	return (rangeLower == value), nil
}
