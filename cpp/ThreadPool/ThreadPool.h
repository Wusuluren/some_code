#pragma once

#include <vector>
#include <Windows.h>


//线程信息类
//ThreadParam_t：线程参数信息结构体
template <typename ThreadParam_t>
class ThreadInfo
{
public:
	HANDLE thread;
	HANDLE start;
	HANDLE stop;
	int id;
	int error;

	ThreadParam_t parameter;
};


//线程池管理类
//ThreadParam_t：线程参数信息结构体
template <typename ThreadParam_t>
class ThreadPoolManager
{
public:
	//构造函数
	ThreadPoolManager();
	//析构函数
	~ThreadPoolManager();
	//创建线程池
	void CreateThreadPool(int number=0, LPTHREAD_START_ROUTINE threadProc=NULL);
	//销毁线程池
	void DestroyThreadPool();
	//重置线程池中进程个数
	void ResetThreadPool(int number=0, LPTHREAD_START_ROUTINE threadProc=NULL);
	//启动线程池所有进程
	void StartThread();
	//等待线程池所有进程完成工作
	void StopThread();
	//休眠线程池所有进程
	void WaitThread();
	//设置线程池某个进程的参数
	void SetParameter(const int &index, ThreadParam_t &p);
	//得到线程池某个进程的参数
	ThreadParam_t& GetParameter(const int &index);
	//设置线程池全部进程的参数
	void SetAllParameter(std::vector<ThreadParam_t> &p);
	//返回线程池中的线程个数
	int Size();

private:
	//线程池管理器
	std::vector<ThreadInfo<ThreadParam_t>> worker;
};


/////////////////////////////////////////////////////////////////////////////////////
//成员函数定义

template <typename ThreadParam_t>
ThreadPoolManager<ThreadParam_t>::ThreadPoolManager()
{
}

template <typename ThreadParam_t>
ThreadPoolManager<ThreadParam_t>::~ThreadPoolManager()
{
}

template <typename ThreadParam_t>
void ThreadPoolManager<ThreadParam_t>::CreateThreadPool(int number, LPTHREAD_START_ROUTINE threadProc)
{
	this->worker.resize(number);

	for (int i = 0; i < this->worker.size(); i++)
	{
		this->worker.at(i).id = i;

		this->worker.at(i).thread = CreateThread(NULL, 0, threadProc, &this->worker.at(i), CREATE_SUSPENDED, NULL);
	}
	for (int i = 0; i < this->worker.size(); i++)
	{
		this->worker.at(i).start = CreateEvent(NULL, true, false, NULL);
		this->worker.at(i).stop = CreateEvent(NULL, true, false, NULL);
	}
}

template <typename ThreadParam_t>
void ThreadPoolManager<ThreadParam_t>::ResetThreadPool(int number, LPTHREAD_START_ROUTINE threadProc)
{
	for (int i = 0; i < this->worker.size(); i++)
	{		
		CloseHandle(this->worker.at(i).stop);
		CloseHandle(this->worker.at(i).start);
		TerminateThread(this->worker.at(i).thread, 0);		
	}


	this->worker.resize(number);

	for (int i = 0; i < this->worker.size(); i++)
	{
		this->worker.at(i).id = i;

		this->worker.at(i).thread = CreateThread(NULL, 0, threadProc, &this->worker.at(i), CREATE_SUSPENDED, NULL);
	}
	for (int i = 0; i < this->worker.size(); i++)
	{
		this->worker.at(i).start = CreateEvent(NULL, true, false, NULL);
		this->worker.at(i).stop = CreateEvent(NULL, true, false, NULL);
	}
}

template <typename ThreadParam_t>
void ThreadPoolManager<ThreadParam_t>::DestroyThreadPool()
{
	for (int i = 0; i < this->worker.size(); i++)
	{		
		CloseHandle(this->worker.at(i).stop);
		CloseHandle(this->worker.at(i).start);
		TerminateThread(this->worker.at(i).thread, 0);		
	}
}

template <typename ThreadParam_t>
void ThreadPoolManager<ThreadParam_t>::StartThread()
{
	for (int i = 0; i < this->worker.size(); i++)
	{
		ResumeThread(this->worker.at(i).thread);
	}
	for (int i = 0; i < this->worker.size(); i++)
	{
		SetEvent(this->worker.at(i).start);
	}
}

template <typename ThreadParam_t>
void ThreadPoolManager<ThreadParam_t>::StopThread()
{
	for (int i = 0; i < this->worker.size(); i++)
	{
		SuspendThread(this->worker.at(i).thread);
	}
}

template <typename ThreadParam_t>
void ThreadPoolManager<ThreadParam_t>::WaitThread()
{
	for (int i = 0; i < this->worker.size(); i++)
	{
		WaitForSingleObject(this->worker.at(i).stop, INFINITE);
		ResetEvent(this->worker.at(i).stop);
	}
}

template <typename ThreadParam_t>
void ThreadPoolManager<ThreadParam_t>::SetParameter(const int &index, ThreadParam_t &p)
{
	this->worker.at(index).parameter = p;
}

template <typename ThreadParam_t>
ThreadParam_t& ThreadPoolManager<ThreadParam_t>::GetParameter(const int &index)
{
	return this->worker.at(index).parameter;
}

template <typename ThreadParam_t>
int ThreadPoolManager<ThreadParam_t>::Size()
{
	return this->worker.size();
}

template <typename ThreadParam_t>
void ThreadPoolManager<ThreadParam_t>::SetAllParameter(std::vector<ThreadParam_t> &p)
{
	int size = p.size();
	for (int i = 0; i < size; i++)
	{
		this->worker.at(i).parameter = p.at(i);
	}	
}


/////////////////////////////////////////////////////////////////////////////////////
//使用示例

////存储文件线程参数结构体
//struct ThrdParam
//{
//	char *buffer;
//};

////
//DWORD WINAPI ThreadProc(LPVOID lpThreadParameter)
//{
//	ThreadInfo<struct ThrdParam> * pParam = (ThreadInfo<struct ThrdParam> *)lpThreadParameter;
//
//	while (true)
//	{
//		WaitForSingleObject(pParam->start, INFINITE);
//		ResetEvent(pParam->start);
//		
//		printf("%s from %d\n", pParam->parameter.buffer, pParam->id);
//
//		SetEvent(pParam->stop);
//	}
//
//	return 0;
//}

////测试程序
//int main()
//{
//	//存储文件的线程池管理器
//	ThreadPoolManager<struct ThrdParam> threadPool;
//
//	//初始化保存文件的线程池
//	printf("创建线程池\n");
//	int threadNumber = 4;
//	threadPool.CreateThreadPool(threadNumber, ThreadProc);
//
//	//重置保存文件的线程池中线程的参数信息
//	printf("设置线程池中各线程的参数\n");
//	vector<struct ThrdParam> thrdParam(threadNumber);
//	for (int i = 0; i < thrdParam.size(); i++)
//	{
//		thrdParam.at(i).buffer = (char *)malloc(sizeog(char) * 16);
//		strcpy(thrdParam.at(i).buffer, "hello");
//	}
//	threadPool.SetAllParameter(thrdParam);
//
//	printf("启动线程池\n");
//	threadPool.StartThread();
//	printf("等待线程池\n");
//	threadPool.WaitThread();
//	Sleep(1000);
//	printf("暂停线程池\n");
//	threadPool.StopThread();
//
////需要手动清理线程参数信息
//	for (int i = 0; i < threadPool.Size(); i++)
//	{
//		struct ThrdParam p = threadPool.GetParameter(i);
//		if (p.buffer)
//		{
//			free(p.buffer);
//		}
//	}
//
//	printf("销毁线程池\n");
//	threadPool.DestroyThreadPool();
//
//	return 0;
//}