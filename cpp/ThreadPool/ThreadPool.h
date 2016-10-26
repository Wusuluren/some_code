#pragma once

#include <Windows.h>


//线程池管理类
//ThreadInfo_t：线程信息结构体
//ThreadParam_t：线程参数信息结构体
template <typename ThreadInfo_t, typename ThreadParam_t>
class ThreadPoolManager
{
public:
	//构造函数
	ThreadPoolManager();
	//析构函数
	~ThreadPoolManager();
	//创建进程池
	void CreateThreadPool(int number=0, LPTHREAD_START_ROUTINE threadProc=NULL);
	//销毁进程池
	void DestroyThreadPool();
	//重置进程池中进程个数
	void ResetThreadPool(int number=0, LPTHREAD_START_ROUTINE threadProc=NULL);
	//启动进程池所有进程
	void StartThread();
	//等待进程池所有进程完成工作
	void StopThread();
	//休眠进程池所有进程
	void WaitThread();
	//设置进程池所有进程的参数
	void SetParameter(vector<ThreadParam_t> p);

private:
	std::vector<ThreadInfo_t> worker;//线程池管理器
};


/////////////////////////////////////////////////////////////////////////////////////
//成员函数定义

template <typename ThreadInfo_t, typename ThreadParam_t>
ThreadPoolManager<ThreadInfo_t, ThreadParam_t>::ThreadPoolManager()
{
}

template <typename ThreadInfo_t, typename ThreadParam_t>
ThreadPoolManager<ThreadInfo_t, ThreadParam_t>::~ThreadPoolManager()
{
}

template <typename ThreadInfo_t, typename ThreadParam_t>
void ThreadPoolManager<ThreadInfo_t, ThreadParam_t>::CreateThreadPool(int number, LPTHREAD_START_ROUTINE threadProc)
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

template <typename ThreadInfo_t, typename ThreadParam_t>
void ThreadPoolManager<ThreadInfo_t, ThreadParam_t>::ResetThreadPool(int number, LPTHREAD_START_ROUTINE threadProc)
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

template <typename ThreadInfo_t, typename ThreadParam_t>
void ThreadPoolManager<ThreadInfo_t, ThreadParam_t>::DestroyThreadPool()
{
	for (int i = 0; i < this->worker.size(); i++)
	{		
		CloseHandle(this->worker.at(i).stop);
		CloseHandle(this->worker.at(i).start);
		TerminateThread(this->worker.at(i).thread, 0);		
	}
}

template <typename ThreadInfo_t, typename ThreadParam_t>
void ThreadPoolManager<ThreadInfo_t, ThreadParam_t>::StartThread()
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

template <typename ThreadInfo_t, typename ThreadParam_t>
void ThreadPoolManager<ThreadInfo_t, ThreadParam_t>::StopThread()
{
	for (int i = 0; i < this->worker.size(); i++)
	{
		SuspendThread(this->worker.at(i).thread);
	}
}

template <typename ThreadInfo_t, typename ThreadParam_t>
void ThreadPoolManager<ThreadInfo_t, ThreadParam_t>::WaitThread()
{
	for (int i = 0; i < this->worker.size(); i++)
	{
		WaitForSingleObject(this->worker.at(i).stop, INFINITE);
		ResetEvent(this->worker.at(i).stop);
	}
}

template <typename ThreadInfo_t, typename ThreadParam_t>
void ThreadPoolManager<ThreadInfo_t, ThreadParam_t>::SetParameter(vector<ThreadParam_t> p)
{
	for  (int i = 0; i < this->worker.size(); i++)
	{
		this->worker.at(i).parameter.buffer = p.at(i).buffer;
		this->worker.at(i).parameter.bufBytes = p.at(i).bufBytes;
		this->worker.at(i).parameter.fileBytes = p.at(i).fileBytes;
		this->worker.at(i).parameter.fileDir = p.at(i).fileDir;
		this->worker.at(i).error = 0;
	}
}


/////////////////////////////////////////////////////////////////////////////////////
//使用示例

////存储文件线程参数结构体
//struct StoreFileThrdParam
//{
//	char *buffer;
//	ULONG32 bufBytes;
//	ULONG32 fileBytes;
//	CString fileDir;
//};

////存储文件线程信息结构体
//struct StoreFileThrdInfo
//{
//	HANDLE thread;
//	HANDLE start;
//	HANDLE stop;
//	int id;
//	int error;
//
//	struct StoreFileThrdParam parameter;
//};


////DMA接收数据,保存数据到文件的线程,可能会保存多个文件
//DWORD WINAPI StoreFileThreadProc(LPVOID lpThreadParameter)
//{
//	struct StoreFileThrdInfo * pParam = (struct StoreFileThrdInfo *)lpThreadParameter;
//
//	while (true)
//	{
//		WaitForSingleObject(pParam->start, INFINITE);
//		ResetEvent(pParam->start);
//		
//		//线程保存的文件个数
//		int fileNumber = pParam->parameter.bufBytes / pParam->parameter.fileBytes;
//		//线程的参数信息
//		char *pBuf = pParam->parameter.buffer;
//		ULONG32 bufOffset = 0;
//		ULONG32 fileSize = pParam->parameter.fileBytes;
//		//保存文件的目录
//		wchar_t fileDir[MAX_PATH];	
//		wcscpy(fileDir, (wchar_t *)pParam->parameter.fileDir.GetBuffer(pParam->parameter.fileDir.GetLength()));
//
//		for (int cnt = 0; cnt < fileNumber; cnt++)
//		{
//			do {
//				wchar_t filePath[MAX_PATH];	
//				swprintf(filePath, L"%ws\\dma(%d)-thread(%d)-file(%d).bin", fileDir, gDmaC2sCounter, pParam->id, cnt);
//				//DbgLog("%d:线程池运行中，文件名是%S\n", pParam->id, filePath);
//
//				ULONG32 lWritten = 0;
//				HandlePointer hFile((LPCWSTR)filePath);	
//				if (hFile.Error())
//				{
//					DbgError("%d:创建文件出错\n", pParam->id);
//					break;
//				}
//				WriteFile(hFile.get(), pBuf + bufOffset, fileSize, (LPDWORD)&lWritten, NULL);
//				if (!lWritten)
//				{
//					pParam->error = 1;
//					DbgError("%d:写文件出错\n", pParam->id);
//				}
//				bufOffset += fileSize;
//			} while (0);
//		}
//
//		SetEvent(pParam->stop);
//	}
//
//	return 0;
//}