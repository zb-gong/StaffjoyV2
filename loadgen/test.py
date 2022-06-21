import requests

FRONTCACHE_URL="http://127.0.0.1:50009"
ACCOUNT_URL="http://127.0.0.1:50007"
COMPANY_URL="http://127.0.0.1:50008"

response = requests.get(FRONTCACHE_URL+"/v1/companies", verify=False)
companies = response.json()['companies']

response = requests.get(FRONTCACHE_URL+"/v1/companies/" + companies[0]['uuid'] + "/teams", verify=False)
teams = response.json()['teams']
tmp = teams[0].copy()
tmp['day_week_starts'] = 'monday'
response = requests.put(COMPANY_URL+"/v1/companies/" + companies[0]['uuid'] + "/teams/" + tmp['uuid'], json=tmp, verify=False)

response = requests.get(FRONTCACHE_URL+"/v1/companies/" + companies[0]['uuid'] + "/teams/" + teams[1]['uuid']+"/jobs", verify=False) 
jobs = response.json()['jobs']
response = requests.get(FRONTCACHE_URL+"/v1/companies/" + companies[0]['uuid'] + "/teams/" + teams[1]['uuid']+"/jobs/"+jobs[0]['uuid'], verify=False)
tmp_job = jobs[0].copy()
tmp_job['color']='5604FE'
response = requests.put(COMPANY_URL+"/v1/companies/" + companies[0]['uuid'] + "/teams/" + teams[1]['uuid']+"/jobs/"+jobs[0]['uuid'], json=tmp_job, verify=False)
response = requests.get(FRONTCACHE_URL+"/v1/companies/" + companies[0]['uuid'] + "/teams/" + teams[1]['uuid']+"/jobs/"+jobs[0]['uuid'], verify=False)

response = requests.get(FRONTCACHE_URL+"/v1/companies/" + companies[0]['uuid'] + "/teams/" + teams[2]['uuid']+"/jobs/03e0e3a6-ce4a-40ff-60e6-f01c35ce47ab", verify=False)

response = requests.get(FRONTCACHE_URL+"/v1/companies/" + companies[0]['uuid'] + "/admins", verify=False)
admins_uuid = response.json()['admins'][0]['user_uuid']

response = requests.get(FRONTCACHE_URL+"/v1/companies/" + companies[0]['uuid'] + "/teams/" + teams[1]['uuid']+"/workers", verify=False)
