# ec2-run

Use this tool to easily connect to an ec2 server and run a rails console on it.

Note: One very important difference between this tool and the way `fs run`
worked, is that this is run on the actual servers. So there is a chance you may
break a production server using this. For this reason, you may want to connect
to workers.

## ProTips

### Set a default stack name

```bash
$ git config --local ec2-run.stack stage-sphinx
```

Now if you omit the `-s` flag, `stage-sphinx` will automatically be used.

## Contribute

To download and hack on the source code, run:
```
$ go get -u github.com/Fullscreen/ec2-run
$ cd $GOPATH/src/github.com/Fullscreen/ec2-run
$ go build
```
