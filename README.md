# ec2-run

Use this tool to easily connect to an ec2 server and run a rails console on it.

Note: One very important difference between this tool and the way `fs run`
worked, is that this is run on the actual servers. So there is a chance you may
break a production server using this. For this reason, you may want to connect
to workers.

Invoke with `-t` to start a tmux session on the remote server. For a tutorial on
tmux, see: https://danielmiessler.com/study/tmux/

## ProTips

### Set a default stack name

If you don't specify `-s`, the tool will try to figure out a stack name from
your git remotes, but you can set a git config to override that.

```bash
$ git config --local ec2-run.stack stage-sphinx
```

Now if you omit the `-s` flag, `stage-sphinx` will automatically be used.

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
