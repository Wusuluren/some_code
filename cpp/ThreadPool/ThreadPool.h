#pragma once

#include <Windows.h>

#include <vector>
using std::vector;

//进程参数结构体
struct ThreadParameter
{
	char *buffer;
	ULONG32 bytes;
	CString fileDir;
};

//进程信息结构体
struct ThreadInfo
{
	HANDLE thread;
	HANDLE start;
	HANDLE stop;
	int id;
	int error;

	ThreadParameter parameter;
};

//进程池管理类
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
	void SetParameter(vector<struct ThreadParameter> p);

private:
	vector<struct ThreadInfo> worker;//进程池管理器
};