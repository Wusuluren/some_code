
#pragma once

//基本类型指针(int、float等)类
template <typename T>
class BasePointer
{
public:
	//默认构造函数
	BasePointer();
	//构造函数
	BasePointer(int number);
	//拷贝构造函数
	BasePointer(T *p);
	//析构函数
	~BasePointer();
	//返回错误状态
	int Error();
	//返回原始指针
	T *get();
	//索引内容
	T& operator[](int index);
	//默认重置指针
	void reset();
	//重置指针
	void reset(int number);
	//拷贝重置指针
	void reset(T *p);

private:
	T *pointer;//指针
	int error;//错误值
};


//文件指针类
class FilePointer
{
public:
	//默认构造参数
	FilePointer();
	//构造函数
	FilePointer(char *filePath, char *fileMode);
	//拷贝构造函数
	FilePointer(FILE *p);
	//析构函数
	~FilePointer();
	//返回错误状态
	int Error();
	//返回原始指针
	FILE *get();
	//默认重置指针
	void reset();
	//重置指针
	void reset(char *filePath, char *fileMode);
	//拷贝重置指针
	void reset(FILE *p);

private:
	FILE *pointer;//指针
	int error;//错误值
};

//Windows句柄类
class HandlePointer
{
public:
	//默认构造参数
	HandlePointer();
	//构造函数
	HandlePointer(LPCWSTR lpFileName,
		DWORD dwDesiredAccess = (GENERIC_READ | GENERIC_WRITE),
		DWORD dwShareMode = 0,
		LPSECURITY_ATTRIBUTES lpSecurityAttributes = NULL,
		DWORD dwCreationDisposition = CREATE_ALWAYS,
		DWORD dwFlagsAndAttributes = FILE_ATTRIBUTE_NORMAL,
		HANDLE hTemplateFile = NULL
	);
	//析构函数
	~HandlePointer();
	//返回错误状态
	int Error();
	//返回原始指针
	HANDLE get();
	//默认重置指针
	void reset();
	//重置指针
	void reset(
		LPCWSTR lpFileName,
		DWORD dwDesiredAccess = (GENERIC_READ | GENERIC_WRITE),
		DWORD dwShareMode = 0,
		LPSECURITY_ATTRIBUTES lpSecurityAttributes = NULL,
		DWORD dwCreationDisposition = CREATE_ALWAYS,
		DWORD dwFlagsAndAttributes = FILE_ATTRIBUTE_NORMAL,
		HANDLE hTemplateFile = NULL
	);
	//拷贝重置指针
	void reset(HANDLE p);

private:
	HANDLE pointer;//指针
	int error;//错误值
};


///////////////////////////////////////////////////////
//模板类的声明和定义不能分开成多个文件
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
	this->pointer = NULL;
	this->error = 0;
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
T& BasePointer<T>::operator[](int index)
{
	return this->pointer[index];
}

//默认重置指针
template <typename T>
void BasePointer<T>::reset()
{
	this->~BasePointer();
}

//重置指针
template <typename T>
void BasePointer<T>::reset(int number)
{
	this->~BasePointer();

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
	this->~BasePointer();
	this->pointer = p;	
}