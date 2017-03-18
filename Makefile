main: main.go
	go build

test: main.go
	./crawler http://www.lyft.com http://eng.lyft.com http://blog.lyft.com http://help.lyft.com http://take.lyft.com http://ride.lyft.com 2> stderr.log 1> stdout.log
	sort stdout.log -o stdout.log

clean:
	rm crawler stderr.log stdout.log
