#include "stdafx.h"
#include <Windows.h>

//线程实例
DWORD WINAPI StoreFileThreadProc(LPVOID lpThreadParameter)
{
	struct ThreadInfo * pParam = (struct ThreadInfo *)lpThreadParameter;

	while (true)
	{
		WaitForSingleObject(pParam->start, INFINITE);
		ResetEvent(pParam->start);
		
		do 
		{
			wchar_t filePath[MAX_PATH];	
			swprintf(filePath, L"%ws\\dma-%d-%d.bin", (wchar_t *)pParam->parameter.fileDir.GetBuffer(pParam->parameter.fileDir.GetLength()), gDmaC2sCounter, pParam->id);
			//DbgLog("%d:线程池运行中，文件名是%S\n", pParam->id, filePath);

			ULONG32 lWritten = 0;
			HandlePointer hFile((LPCWSTR)filePath);	
			if (hFile.Error())
			{
				DbgError("%d:创建文件出错\n", pParam->id);
				break;
			}
			WriteFile(hFile.get(), pParam->parameter.buffer, pParam->parameter.bytes, (LPDWORD)&lWritten, NULL);
			if (!lWritten)
			{
				pParam->error = 1;
				DbgError("%d:写文件出错\n", pParam->id);
			}

		} while (0);

		SetEvent(pParam->stop);
	}

	return 0;
}

ThreadPoolManager::ThreadPoolManager()
{
}

ThreadPoolManager::~ThreadPoolManager()
{
}

void ThreadPoolManager::CreateThreadPool(int number, LPTHREAD_START_ROUTINE threadProc)
{
	this->worker.resize(number);

	for (int i = 0; i < this->worker.size(); i++)
	{
		this->worker.at(i).id = i;

		this->worker.at(i).thread = CreateThread(NULL, 0, threadProc, &this->worker.at(i), CREATE_SUSPENDED, NULL);
		//this->worker.at(i).thread = CreateThread(NULL, 0, StoreFileThreadProc, &this->worker.at(i), CREATE_SUSPENDED, NULL);
	}
	for (int i = 0; i < this->worker.size(); i++)
	{
		this->worker.at(i).start = CreateEvent(NULL, true, false, NULL);
		this->worker.at(i).stop = CreateEvent(NULL, true, false, NULL);
	}
}

void ThreadPoolManager::ResetThreadPool(int number, LPTHREAD_START_ROUTINE threadProc)
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
		//this->worker.at(i).thread = CreateThread(NULL, 0, StoreFileThreadProc, &this->worker.at(i), CREATE_SUSPENDED, NULL);
	}
	for (int i = 0; i < this->worker.size(); i++)
	{
		this->worker.at(i).start = CreateEvent(NULL, true, false, NULL);
		this->worker.at(i).stop = CreateEvent(NULL, true, false, NULL);
	}
}

void ThreadPoolManager::DestroyThreadPool()
{
	for (int i = 0; i < this->worker.size(); i++)
	{		
		CloseHandle(this->worker.at(i).stop);
		CloseHandle(this->worker.at(i).start);
		TerminateThread(this->worker.at(i).thread, 0);		
	}
}

void ThreadPoolManager::StartThread()
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

void ThreadPoolManager::StopThread()
{
	for (int i = 0; i < this->worker.size(); i++)
	{
		SuspendThread(this->worker.at(i).thread);
	}
}

void ThreadPoolManager::WaitThread()
{
	for (int i = 0; i < this->worker.size(); i++)
	{
		WaitForSingleObject(this->worker.at(i).stop, INFINITE);
		ResetEvent(this->worker.at(i).stop);
	}
}

void ThreadPoolManager::SetParameter(vector<struct ThreadParameter> p)
{
	for  (int i = 0; i < this->worker.size(); i++)
	{
		this->worker.at(i).parameter.buffer = p.at(i).buffer;
		this->worker.at(i).parameter.bytes = p.at(i).bytes;
		this->worker.at(i).parameter.fileDir = p.at(i).fileDir;

		this->worker.at(i).error = 0;
	}
}