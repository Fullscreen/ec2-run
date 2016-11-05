package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
)

const version = "1.0.0"

var (
	versionFlag      bool
	yesFlag          bool
	tmuxFlag         bool
	listSessionsFlag bool
	profileFlag      string
	regionFlag       string
	stackFlag        string
	sessionNameFlag  string
)

func main() {
	flag.BoolVar(&versionFlag, "version", false, "Print version number ("+version+")")
	flag.BoolVar(&yesFlag, "y", false, `Automatically pick the oldest server if presented with more than one.`)
	flag.BoolVar(&tmuxFlag, "t", false, `Use tmux. Recommended if your ssh session is critical or you are running a big migration.`)
	flag.BoolVar(&listSessionsFlag, "l", false, `List tmux sessions running on the server.`)
	flag.StringVar(&profileFlag, "profile", "default", `The AWS profile to use.`)
	flag.StringVar(&regionFlag, "region", "us-east-1", `The AWS region to use.`)
	flag.StringVar(&stackFlag, "s", "", `The stack name.`)
	flag.StringVar(&sessionNameFlag, "n", "", `The name of the tmux session. You can use this to open another person's session.`)
	flag.Parse()

	if versionFlag {
		fmt.Println(version)
		os.Exit(0)
	}

	if stackFlag == "" {
		fmt.Println("Missing stack name. You can use a CloudFormation stack name from https://console.aws.amazon.com/cloudformation/home?region=us-east-1#/stacks?filter=active")
		os.Exit(1)
	}
	matcher := fmt.Sprintf("*%s*", stackFlag)

	command := strings.Join(flag.Args(), " ")
	if command == "" {
		command = "rails console"
		fmt.Printf("Missing command, will use '%s'.\n", command)
	}

	// Get credentials based on profile flag
	// TODO: Better to modify AWS_PROFILE env var?
	usr, _ := user.Current()
	credentialsPath := fmt.Sprintf("%s/.aws/credentials", usr.HomeDir)
	credentialsProvider := credentials.NewSharedCredentials(credentialsPath, profileFlag)
	creds, err := credentialsProvider.Get()
	check(err)
	fmt.Printf("Using access key %s from profile \"%s\".\n", creds.AccessKeyID, profileFlag)

	// Create session
	sess, err := session.NewSession(&aws.Config{
		Region:      &regionFlag,
		Credentials: credentialsProvider,
	})
	check(err)

	// ec2 describe-instances
	ec2Client := ec2.New(sess)
	respDescribeInstances, err := ec2Client.DescribeInstances(&ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{
			{
				Name: aws.String("tag:aws:cloudformation:stack-name"),
				Values: []*string{
					aws.String(matcher),
				},
			},
		},
	})
	check(err)

	if len(respDescribeInstances.Reservations) == 0 {
		fmt.Printf("No instances matched '%s'.", matcher)
		return
	}
	fmt.Printf("Found %d instances matching '%s'..\n", len(respDescribeInstances.Reservations[0].Instances), matcher)
	// TODO: Print a table that the user can select a server from
	ip := *respDescribeInstances.Reservations[0].Instances[0].PrivateIpAddress

	var cmd string
	if listSessionsFlag {
		cmd = "tmux list-sessions"
	} else if tmuxFlag {
		if sessionNameFlag == "" {
			sessionNameFlag = fmt.Sprintf("console-%s", os.Getenv("USER"))
		}
		fmt.Printf("Using tmux session name '%s'.", sessionNameFlag)
		cmd = fmt.Sprintf(`
export SESSION_NAME="%s"
export COMMAND="%s"
tmux has-session -t "$SESSION_NAME"
if [[ $? -eq 0 ]]; then
  read -p "There is a session with the name $SESSION_NAME already. Do you want to (a)ttach to or (k)ill the session? " WAT
  if [[ "$WAT" == "a" ]]; then
    tmux attach-session -t "$SESSION_NAME"
  elif [[ "$WAT" == "k" ]]; then
    tmux kill-session -t "$SESSION_NAME"
  else
    echo "Did not understand '$WAT'"
  fi
else
  tmux new-session -s "$SESSION_NAME" "sudo -su deploy -- bash -i -c \"cd /srv; source app-env; echo 'Running: $COMMAND... Press Ctrl+B then D to detach your session.'; $COMMAND\""
fi`, sessionNameFlag, command)
	} else {
		cmd = fmt.Sprintf(`sudo -su deploy -- bash -i -c "cd /srv; source app-env; echo 'Running: %s'; %s"`, command, command)
	}

	sshOptions := []string{
		ip,
		"-o", "LogLevel=ERROR",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "ConnectTimeout=10",
		"-t",
		cmd,
	}
	// fmt.Println(sshOptions)
	fmt.Printf("Opening ssh session to: %s...\n", ip)

	sshCmd := exec.Command("ssh", sshOptions...)
	sshCmd.Stdin = os.Stdin
	sshCmd.Stdout = os.Stdout
	sshCmd.Stderr = os.Stderr
	sshCmd.Start()
	check(sshCmd.Wait())
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
