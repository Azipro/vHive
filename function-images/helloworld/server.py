from concurrent import futures
import logging

import grpc

import psutil
import os,datetime,time

import helloworld_pb2
import helloworld_pb2_grpc

responses = ["record_response", "replay_response"]

def getMemCpu():
    data = psutil.virtual_memory()
    total = data.total
    free = data.available
    memory =  "Memory usage:%d"%(int(round(data.percent)))+"%"+"  "
    cpu = "CPU:%0.2f"%psutil.cpu_percent(interval=1)+"%"
    return memory+cpu
        
class Greeter(helloworld_pb2_grpc.GreeterServicer):

    def SayHello(self, request, context):
        if request.name == "record":
            msg = 'Hello, %s!' % responses[0]
        elif request.name == "replay":
            msg = 'Hello, %s!' % responses[1]
        else:
            msg = 'Hello, %s!' % request.name
        
        msg += getMemCpu()
        return helloworld_pb2.HelloReply(message=msg)


def serve():
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=1))
    helloworld_pb2_grpc.add_GreeterServicer_to_server(Greeter(), server)
    server.add_insecure_port('[::]:50051')
    server.start()
    server.wait_for_termination()


if __name__ == '__main__':
    logging.basicConfig()
    serve()
