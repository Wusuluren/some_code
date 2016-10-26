#include <stdio.h>
#include "stdafx.h"
#include "TestCtpPciePointer.h"


//////////////////////////////////////////////////////
//文件指针类

//默认构造函数
FilePointer::FilePointer():
	error(0)
{
	this->pointer = NULL;	
}

//构造函数
FilePointer::FilePointer(char *filePath, char *fileMode):
	error(0)	
{
	this->pointer = fopen(filePath, fileMode);
	if (NULL == this->pointer)
	{
		this->error = 1;
	}		
}

//拷贝构造函数
FilePointer::FilePointer(FILE *&p)
{
	this->pointer = p;
}

//析构函数
FilePointer::~FilePointer()
{
	if (NULL != this->pointer)
	{
		fclose(this->pointer);
		//DbgLog("关闭文件\n");
	}
	this->pointer = NULL;
	this->error = 0;
}

//返回错误状态
int FilePointer::Error()
{
	return this->error;
}

//返回原始指针
FILE *FilePointer::get()
{
	return this->pointer;
}

//默认重置指针
void FilePointer::reset()
{
	this->~FilePointer();
}

//重置指针
void FilePointer::reset(char *filePath, char *fileMode)
{
	this->~FilePointer();

	this->pointer = fopen(filePath, fileMode);
	if (NULL == this->pointer)
	{
		this->error = 1;
	}	
}

//拷贝重置指针
void FilePointer::reset(FILE *&p)
{
	this->~FilePointer();
	this->pointer = p;
}


//////////////////////////////////////////////////////
//Windows句柄类

//默认构造函数
HandlePointer::HandlePointer():
	error(0)
{
	this->pointer = INVALID_HANDLE_VALUE;	
}

//构造函数
HandlePointer::HandlePointer(    
	const LPCWSTR &lpFileName,
    const DWORD &dwDesiredAccess,
    const DWORD &dwShareMode,
    const LPSECURITY_ATTRIBUTES &lpSecurityAttributes,
    const DWORD &dwCreationDisposition,
    const DWORD &dwFlagsAndAttributes,
    const HANDLE &hTemplateFile):
	error(0)	
{
	//DbgLog("*创建文件%S*\n", lpFileName);

	this->pointer = CreateFile(
		lpFileName,
		dwDesiredAccess,
		dwShareMode,
		lpSecurityAttributes,
		dwCreationDisposition,
		dwFlagsAndAttributes,
		hTemplateFile
	);
	if (INVALID_HANDLE_VALUE == this->pointer)
	{
		//DbgLog("*错误代码是%d*\n", GetLastError());
		this->error = 1;
	}		
}

//析构函数
HandlePointer::~HandlePointer()
{
	if (INVALID_HANDLE_VALUE != this->pointer)
	{
		CloseHandle(this->pointer);
		//DbgLog("关闭Windows句柄\n");
	}
	this->pointer = INVALID_HANDLE_VALUE;
	this->error = 0;
}

//返回错误状态
int HandlePointer::Error()
{
	return this->error;
}

//返回原始指针
HANDLE HandlePointer::get()
{
	return this->pointer;
}

//默认重置指针
void HandlePointer::reset()
{
	this->~HandlePointer();
}

//重置指针
void HandlePointer::reset(
	const LPCWSTR &lpFileName,
    const DWORD &dwDesiredAccess,
    const DWORD &dwShareMode,
    const LPSECURITY_ATTRIBUTES &lpSecurityAttributes,
    const DWORD &dwCreationDisposition,
    const DWORD &dwFlagsAndAttributes,
    const HANDLE &hTemplateFile)
{
	this->~HandlePointer();

	this->pointer = CreateFile(
		lpFileName,
		dwDesiredAccess,
		dwShareMode,
		lpSecurityAttributes,
		dwCreationDisposition,
		dwFlagsAndAttributes,
		hTemplateFile
	);
	if (INVALID_HANDLE_VALUE == this->pointer)
	{
		this->error = 1;
	}	
}

//拷贝重置指针
void HandlePointer::reset(const HANDLE &p)
{
	this->~HandlePointer();
	this->pointer = p;
}