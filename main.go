package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/olekukonko/tablewriter"
	"github.com/tcnksm/go-gitconfig"
)

const version = "1.0.0"

var (
	versionFlag      bool
	verboseFlag      bool
	yesFlag          bool
	tmuxFlag         bool
	listSessionsFlag bool
	listStacksFlag   bool
	profileFlag      string
	regionFlag       string
	stackFlag        string
	sessionNameFlag  string
)

func main() {
	flag.BoolVar(&versionFlag, "version", false, "Print version number ("+version+")")
	flag.BoolVar(&verboseFlag, "v", false, `Be more verbose.`)
	flag.BoolVar(&yesFlag, "y", false, `Automatically pick the oldest server if presented with more than one.`)
	flag.BoolVar(&tmuxFlag, "t", false, `Use tmux. Recommended if your ssh session is critical or you are running a big migration.`)
	flag.BoolVar(&listSessionsFlag, "l", false, `List tmux sessions running on the server.`)
	flag.BoolVar(&listStacksFlag, "ls", false, `List stacks (optionally with a filter).`)
	flag.StringVar(&profileFlag, "profile", "default", `The AWS profile to use.`)
	flag.StringVar(&regionFlag, "region", "us-east-1", `The AWS region to use.`)
	flag.StringVar(&stackFlag, "s", "", `The stack name.`)
	flag.StringVar(&sessionNameFlag, "n", "", `The name of the tmux session. You can use this to open another person's session.`)
	flag.Parse()

	if versionFlag {
		fmt.Println(version)
		os.Exit(0)
	}

	// Get credentials based on profile flag
	// TODO: Better to modify AWS_PROFILE env var?
	usr, _ := user.Current()
	credentialsPath := fmt.Sprintf("%s/.aws/credentials", usr.HomeDir)
	credentialsProvider := credentials.NewSharedCredentials(credentialsPath, profileFlag)
	if verboseFlag {
		creds, err := credentialsProvider.Get()
		check(err)
		fmt.Printf("Using access key %s from profile \"%s\".\n", creds.AccessKeyID, profileFlag)
	}

	// Create session
	sess, err := session.NewSession(&aws.Config{
		Region:      &regionFlag,
		Credentials: credentialsProvider,
	})
	check(err)

	if listStacksFlag {
		stackName := flag.Arg(0)
		cfClient := cloudformation.New(sess)
		var nextToken *string
		var stacks []string
		for {
			respDescribeStacks, err2 := cfClient.DescribeStacks(&cloudformation.DescribeStacksInput{
				NextToken: nextToken,
			})
			check(err2)
			for _, stack := range respDescribeStacks.Stacks {
				if stackName == "" || strings.Contains(*stack.StackName, stackName) {
					stacks = append(stacks, *stack.StackName)
				}
			}
			if respDescribeStacks.NextToken == nil {
				break
			}
			nextToken = respDescribeStacks.NextToken
		}
		sort.Strings(stacks)
		for _, stack := range stacks {
			fmt.Println(stack)
		}
		os.Exit(0)
	}

	if stackFlag == "" {
		stack, err2 := gitconfig.Local("ec2-run.stack")
		if err2 == nil {
			fmt.Printf("Using stack from git config: %s\n", stack)
			stackFlag = stack
		} else {
			flag.PrintDefaults()
			fmt.Println("\nError: Missing stack name.")
			fmt.Printf("Use -ls to list stacks in the command line or visit https://console.aws.amazon.com/cloudformation/home?region=%s#/stacks?filter=active\n", regionFlag)
			fmt.Println("See README.md on how to set a default stack.")
			os.Exit(1)
		}
	}
	matcher := fmt.Sprintf("*%s*", stackFlag)

	command := strings.Join(flag.Args(), " ")
	if command == "" && !listSessionsFlag {
		command = "rails console"
		if verboseFlag {
			fmt.Printf("Missing command, will run '%s'.\n", command)
		}
	}

	// ec2 describe-instances
	ec2Client := ec2.New(sess)
	respDescribeInstances, err := ec2Client.DescribeInstances(&ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{
			{
				Name: aws.String("tag:aws:cloudformation:stack-name"),
				// Name: aws.String("tag:Name"),
				Values: []*string{
					aws.String(matcher),
				},
			},
			{
				Name: aws.String("instance-state-name"),
				Values: []*string{
					aws.String("running"),
				},
			},
		},
	})
	check(err)

	var instances []*ec2.Instance

	for _, reservation := range respDescribeInstances.Reservations {
		for _, instance := range reservation.Instances {
			instances = append(instances, instance)
		}
	}

	var instance *ec2.Instance
	switch len(instances) {
	case 0:
		fmt.Printf("No instances matched '%s'.", matcher)
		os.Exit(1)
	case 1:
		fmt.Printf("Found 1 instance matching '%s'.\n", matcher)
		instance = instances[0]
	default:
		sort.Sort(byLaunchTime(instances))
		fmt.Printf("Found %d instances matching '%s':\n", len(instances), matcher)

		if !yesFlag || verboseFlag {
			table := tablewriter.NewWriter(os.Stdout)
			table.SetHeader([]string{"#", "Name", "ID", "Type", "Uptime", "Roles"})
			table.SetBorder(false)
			for i, instance := range instances {
				table.Append([]string{
					strconv.Itoa(i),
					getTag("Name", instance),
					*instance.InstanceId,
					*instance.InstanceType,
					strconv.FormatFloat(time.Now().Sub(*instance.LaunchTime).Hours(), 'f', 1, 64),
					getTag("Roles", instance),
				})
			}
			fmt.Println()
			table.Render()
		}

		// fmt.Println()
		if yesFlag {
			fmt.Printf("Automatically selected %s\n", getTag("Name", instances[0]))
			instance = instances[0]
		} else {
			reader := bufio.NewReader(os.Stdin)
			fmt.Print("Select instance: [0] ")
			text, _ := reader.ReadString('\n')
			if text == "\n" {
				text = "0"
			}
			selection, err2 := strconv.Atoi(strings.TrimRight(text, "\n"))
			if err2 != nil {
				fmt.Println("Error: unable to convert selection to number.")
				os.Exit(1)
			}
			if selection < 0 || selection > len(instances) {
				fmt.Println("Error: index out of range.")
				os.Exit(1)
			}
			instance = instances[selection]
		}
	}
	ip := *instance.PrivateIpAddress

	var cmd string
	if listSessionsFlag {
		cmd = "tmux list-sessions"
		fmt.Println("If you see 'failed to connect to server', this means there are no tmux sessions open.")
	} else if tmuxFlag {
		if sessionNameFlag == "" {
			sessionNameFlag = fmt.Sprintf("console-%s", os.Getenv("USER"))
		}
		if verboseFlag {
			fmt.Printf("Using tmux session name '%s'.\n", sessionNameFlag)
		}
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
  tmux new-session -s "$SESSION_NAME" "sudo -su deploy -- bash -i -c \"cd /srv; source app-env; echo 'Running: $COMMAND... Press Ctrl+B then D to detach your session.'; echo; $COMMAND\""
fi`, sessionNameFlag, command)
	} else {
		cmd = fmt.Sprintf(`sudo -su deploy -- bash -i -c "cd /srv; source app-env; echo 'Running: %s'; echo; %s"`, command, command)
	}

	sshOptions := []string{
		ip,
		"-o", "LogLevel=ERROR",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "ConnectTimeout=10",
		"-t",
	}
	if verboseFlag {
		sshOptions = append(sshOptions, "-v")
	}
	sshOptions = append(sshOptions, cmd)
	if verboseFlag {
		fmt.Println("ssh", sshOptions)
	}
	fmt.Printf("Opening ssh session to: %s...\n", ip)

	sshCmd := exec.Command("ssh", sshOptions...)
	sshCmd.Stdin = os.Stdin
	sshCmd.Stdout = os.Stdout
	sshCmd.Stderr = os.Stderr
	sshCmd.Start()
	err = sshCmd.Wait()
	if err != nil {
		if err.Error() == "exit status 255" {
			fmt.Println("Please make sure you are connected to the VPN if you are away from the office.")
		} else {
			check(err)
		}
	}
}

func getTag(name string, instance *ec2.Instance) string {
	for _, t := range instance.Tags {
		if *t.Key == name {
			return *t.Value
		}
	}
	return ""
}

type byLaunchTime []*ec2.Instance

func (s byLaunchTime) Len() int {
	return len(s)
}
func (s byLaunchTime) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
func (s byLaunchTime) Less(i, j int) bool {
	return s[i].LaunchTime.Before(*s[j].LaunchTime)
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
