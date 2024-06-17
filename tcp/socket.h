#pragma once
#include <stdbool.h>
#include <arpa/inet.h>


enum Type {
    Heart,
    Message
};

// 初始化一个套接字
int initSocket();
// 初始化sockaddr结构体
void initSockaddr(struct sockaddr* addr, unsigned port, const char* ip);
// 设置监听
int setListen(int lfd, unsigned port);
// 接收客户端连接
int acceptConnect(int lfd, struct sockaddr* addr);
// 连接服务器
int connectToHost(int fd, unsigned port, const char* ip);
// 读出指定字节数
int readn(int fd, char* buffer, int sise);
// 写入指定字节数
int writen(int fd, const char* buffer, int length);
// 发送数据
int sendMessage(int fd, const char* buffer, int length, enum Type t);
// 接收数据
int recvMessage(int fd, char** buffer, enum Type* t);
