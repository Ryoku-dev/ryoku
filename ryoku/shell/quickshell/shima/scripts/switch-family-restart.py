#!/usr/bin/env python3
import os, sys
os.execvp('ryoku-shell', ['ryoku-shell', 'shima-family'] + sys.argv[1:])
