const baseURL = import.meta.env.VITE_API_URL ?? '/api/v1'

export async function api<T>(path:string, init:RequestInit={}):Promise<T>{
  const token=localStorage.getItem('desk_access_token')
  const response=await fetch(`${baseURL}${path}`,{...init,credentials:'include',headers:{Accept:'application/json',...(token?{Authorization:`Bearer ${token}`}:{}) ,...init.headers}})
  if(!response.ok) throw new Error(`API ${response.status}`)
  return response.json() as Promise<T>
}
export type Product={id:string;key:string;name:string;description:string;enabled:boolean;health:string}
export type Service={id:string;product_id:string;key:string;name:string;kind:string;enabled:boolean;status:string;latency_ms:number|null}
export type Alert={id:string;priority:string;state:string;title:string;message:string}
export type Device={id:string;device_id:string;name:string;state:string;firmware_version:string|null}
