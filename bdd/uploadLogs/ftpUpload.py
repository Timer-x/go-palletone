#!/usr/bin/env python
# -*- coding: utf-8 -*-

import os
import pexpect
import sys

logPath = "/home/travis/gopath/src/github.com/palletone/go-palletone/bdd/logs"
os.chdir(logPath)
# login
child = pexpect.spawnu("ftp 98.142.130.141")
child.logfile = sys.stdout
print child.after
child.expect(u'(?i)Name.*.')
child.sendline("ftpuser")
print child.after
child.expect(u"(?i)Password")
child.sendline("123456")
print child.after
# login succeed
child.expect(u"ftp>")
child.sendline("cd bddlogs")
print child.after
# upload logs
child.expect(u"ftp>")
createTransLogName = "createTrans.log.html"
# createTransLogName = "log.txt"
# ccinvokeLogName = "ccinvoke.log.html"
# DigitalIdentityCertLogName = "DigitalIdentityCert.log.html"
putStr = "put "+logPath+"/"+createTransLogName+" "+createTransLogName
child.sendline("put /home/travis/gopath/src/github.com/palletone/go-palletone/bdd/node/log/all.log output.xml")
print putStr
try:
	child.expect(u"(?i).*complete.*")
	print "=== upload succed ==="
except:
    print "==== setting passive mode ==="
    child.sendline('quote pasv')
    child.sendline('passive')
    child.after
    child.sendline("put /home/travis/gopath/src/github.com/palletone/go-palletone/bdd/node/log/all.log output.xml")
    print putStr
try:
    child.expect(u"(?i).*complete.*")
    print "=== upload succeed ==="
except:
    print "=== upload failed === "

child.expect(u"ftp>")
child.sendline("bye")
print child.after
child.close()
