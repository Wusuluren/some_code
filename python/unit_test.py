from filter import *
import unittest

class FilterUnitTesst(unittest.TestCase):
	def testInit(self):
		os.chdir('UnitTestFile')
		Filter(FindFile(os.curdir))

if __name__ == '__main__':
	unittest.main()