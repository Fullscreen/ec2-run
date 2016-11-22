# ec2-run

Use this tool to easily connect to an EC2 server and run a rails console on it
(or whatever command you specify).

Note: What you run using this tool will be run on the EC2 server you select,
so if you run a destructive action on a production server, you will break that
production server.

Invoke with `-t` to start a tmux session on the remote server. For a tutorial on
tmux, see: https://danielmiessler.com/study/tmux/

## Install

You can install this tool by using [our Homebrew tap](https://github.com/Fullscreen/homebrew-tap):

```shell
brew tap fullscreen/tap
brew install ec2-run
```

## ProTips

### Stack name

This tool assumes that you want to run commands on EC2 instances that are
launched from CloudFormation stacks, so it looks up instances based on the tag
`tag:aws:cloudformation:stack-name`.

If you don't specify `-s`, the tool will try to figure out a stack name from
your git remotes, but you can set a git config to override that.

```bash
$ git config --local ec2-run.stack stage-datascience
```

Now if you omit the `-s` flag, `stage-datascience` will automatically be used.

Revert with:

```bash
$ git config --local --unset ec2-run.stack
```

### Use tmux by default

```bash
$ git config --global ec2-run.tmux true
```

Revert with:

```bash
$ git config --global --unset ec2-run.tmux
```

## Contribute

To download and hack on the source code, run:
```
$ go get -u github.com/Fullscreen/ec2-run
$ cd $GOPATH/src/github.com/Fullscreen/ec2-run
$ go build
```
