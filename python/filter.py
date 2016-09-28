import os
import re
import shutil

def FindFile():
	fileList = []
	curDirFiles = os.listdir()
	for f in curDirFiles:
		if [] != re.findall('.[c|cpp]$', f):
			fileList.append(f)

	return fileList

def Filter(fileList):
	filterDir = 'filter_out'
	if os.path.exists(filterDir):
		shutil.rmtree(filterDir)
	if fileList:
		os.mkdir(filterDir)

	for f in fileList:
		brackStack = []
		srcFileName = f
		destFileName = filterDir+'/'+'out_'+f
		srcFile = open(srcFileName, 'r')
		destFile = open(destFileName, 'w')
		srcCode = srcFile.read()
		destCode = ''

		for char in srcCode:
			if '{' == char:
				if 0 == len(brackStack):
					destCode = destCode + '{'
				brackStack.append(char)
			elif '}' == char:
				brackStack.pop()
				if 0 == len(brackStack):
					destCode = destCode + '}'
			elif 0 == len(brackStack):
				destCode = destCode + char

		destFile.write(destCode)

		srcFile.close()
		destFile.close()

if __name__ == '__main__':	
	Filter(FindFile())