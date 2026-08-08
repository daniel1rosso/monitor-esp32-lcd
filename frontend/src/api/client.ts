const baseURL = import.meta.env.VITE_API_URL ?? '/api/v1'

export class APIError extends Error {
  constructor(public status:number){super(`API ${status}`)}
}

let refreshing:Promise<string>|null=null

export async function api<T>(path:string, init:RequestInit={}):Promise<T>{
  let token=localStorage.getItem('desk_access_token')
  let response=await request(path,init,token)
  if(response.status===401&&!path.startsWith('/auth/')){
    try{
      refreshing??=refreshAccessToken().finally(()=>{refreshing=null})
      token=await refreshing
      response=await request(path,init,token)
    }catch{localStorage.removeItem('desk_access_token')}
  }
  if(!response.ok) throw new APIError(response.status)
  if(response.status===204) return undefined as T
  return response.json() as Promise<T>
}
function request(path:string,init:RequestInit,token:string|null){return fetch(`${baseURL}${path}`,{...init,credentials:'include',headers:{Accept:'application/json',...(token?{Authorization:`Bearer ${token}`}:{}) ,...init.headers}})}
async function refreshAccessToken(){const response=await fetch(`${baseURL}/auth/refresh`,{method:'POST',credentials:'include',headers:{Accept:'application/json'}});if(!response.ok)throw new APIError(response.status);const result=await response.json() as {access_token:string};localStorage.setItem('desk_access_token',result.access_token);return result.access_token}
export async function login(email:string,password:string){
  return api<{access_token:string;user:{email:string;role:string}}>('/auth/login',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({email,password})})
}
export async function logout(){return api<void>('/auth/logout',{method:'POST'})}
export type Product={id:string;key:string;name:string;description:string;enabled:boolean;health:string}
export type Service={id:string;product_id:string;key:string;name:string;kind:string;endpoint:string|null;enabled:boolean;status:string;latency_ms:number|null}
export type Alert={id:string;priority:string;state:string;title:string;message:string}
export type Device={id:string;device_id:string;name:string;state:string;firmware_version:string|null}
export type Deployment={id:string;product_id:string|null;repository:string;branch:string;workflow:string;status:string;commit_sha:string;commit_message:string;author:string;started_at:string;finished_at:string|null;html_url:string|null}
export type MarketQuote={provider:string;symbol:string;quote_asset:string;bid:number|null;ask:number|null;last:number;change_percent:number;observed_at:string;stale:boolean}
export type Weather={location:string;temperature_c:number;apparent_temperature_c:number;humidity_percent:number|null;weather_code:number;wind_kmh:number|null;daily_min_c:number|null;daily_max_c:number|null;precipitation_probability_percent:number|null;observed_at:string;stale:boolean}
