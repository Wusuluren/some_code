#include <stdio.h>
#include "stdafx.h"
#include "PointerClass.h"

///////////////////////////////////////////////////////
//基本类型指针(int、float等)类

//默认构造函数
template <typename T>
BasePointer<T>::BasePointer():
	pointer(NULL), error(0)
{
}

//构造函数
template <typename T>
BasePointer<T>::BasePointer(int number):
	pointer(NULL), error(0)
{
	this->pointer = (T *)malloc(sizeof(T) * number);
	if (NULL == this->pointer)
	{
		this->error = 1;
	}
}

//拷贝构造函数
template <typename T>
BasePointer<T>::BasePointer(T *p):
	pointer(NULL), error(0)
{
	this->pointer = p;
}

//析构函数
template <typename T>
BasePointer<T>::~BasePointer()
{
	if (NULL != this->pointer)
	{
		free(this->pointer);
		//DbgLog("释放内存\n");
	}
}

//返回错误状态
template <typename T>
int BasePointer<T>::Error()
{
	return this->error;
}

//返回原始指针
template <typename T>
T *BasePointer<T>::get()
{
	return this->pointer;
}

//索引内容
template <typename T>
T BasePointer<T>::operator[](int index)
{
	return this->pointer[index];
}

//默认重置指针
template <typename T>
void BasePointer<T>::reset()
{
	if (NULL != this->pointer)
	{
		free(this->pointer);
		//DbgLog("释放内存\n");
	}
	this->error = 0;

	this->poiner = NULL;
}

//重置指针
template <typename T>
void BasePointer<T>::reset(int number)
{
	if (NULL != this->pointer)
	{
		free(this->pointer);
		//DbgLog("释放内存\n");
	}
	this->error = 0;

	this->pointer = (T *)malloc(sizeof(T) * number);
	if (NULL == this->pointer)
	{
		this->error = 1;
	}
}

//拷贝重置指针
template <typename T>
void BasePointer<T>::reset(T *p)
{
	if (NULL != this->pointer)
	{
		free(this->pointer);
		//DbgLog("释放内存\n");
	}
	this->error = 0;

	this->pointer = p;	
}



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
FilePointer::FilePointer(FILE *p)
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
	if (NULL != this->pointer)
	{
		fclose(this->pointer);
		//DbgLog("关闭文件\n");
	}
	this->error = 0;
}

//重置指针
void FilePointer::reset(char *filePath, char *fileMode)
{
	if (NULL != this->pointer)
	{
		fclose(this->pointer);
		//DbgLog("关闭文件\n");
	}
	this->error = 0;

	this->pointer = fopen(filePath, fileMode);
	if (NULL == this->pointer)
	{
		this->error = 1;
	}	
}

//拷贝重置指针
void FilePointer::reset(FILE *p)
{
	if (NULL != this->pointer)
	{
		fclose(this->pointer);
		//DbgLog("关闭文件\n");
	}
	this->error = 0;

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
	LPCWSTR lpFileName,
    DWORD dwDesiredAccess,
    DWORD dwShareMode,
    LPSECURITY_ATTRIBUTES lpSecurityAttributes,
    DWORD dwCreationDisposition,
    DWORD dwFlagsAndAttributes,
    HANDLE hTemplateFile):
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
	if (INVALID_HANDLE_VALUE != this->pointer)
	{
		CloseHandle(this->pointer);
		//DbgLog("关闭Windows句柄\n");
		this->pointer = INVALID_HANDLE_VALUE;
	}
	this->error = 0;
}

//重置指针
void HandlePointer::reset(
	LPCWSTR lpFileName,
    DWORD dwDesiredAccess,
    DWORD dwShareMode,
    LPSECURITY_ATTRIBUTES lpSecurityAttributes,
    DWORD dwCreationDisposition,
    DWORD dwFlagsAndAttributes,
    HANDLE hTemplateFile)
{
	if (INVALID_HANDLE_VALUE != this->pointer)
	{
		CloseHandle(this->pointer);
		//DbgLog("关闭Windows句柄\n");
	}
	this->error = 0;

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
void HandlePointer::reset(HANDLE p)
{
	if (INVALID_HANDLE_VALUE != this->pointer)
	{
		CloseHandle(this->pointer);
		//DbgLog("关闭Windows句柄\n");
	}
	this->error = 0;

	this->pointer = p;
}