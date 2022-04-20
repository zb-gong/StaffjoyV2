import json
import requests
import random
import sys
import grpc
import inspect
import time
import gevent
import ast

from locust.contrib.fasthttp import FastHttpUser
from locust import task, events, constant, between, tag
from locust.runners import STATE_STOPPING, STATE_STOPPED, STATE_CLEANUP, WorkerRunner

class SharedData(object):
    initialized = False
    accounts = None
    companies = None
    teams = None
    workers = None
    jobs = None

COMPANY_URL="http://127.0.0.1:50007"
ACCOUNT_URL="http://127.0.0.1:50006"

class User(FastHttpUser):
    wait_time = between(0.25, 0.5)

    host = 'https://test.github.io'

    def on_start(self):
        response = requests.get(COMPANY_URL + "/v1/companies", verify=False)
        SharedData.companies = response.json()['companies']
        print("Companies:", len(SharedData.companies))

        response = requests.get(ACCOUNT_URL + "/v1/accounts", verify=False)
        SharedData.accounts = response.json()['accounts']
        print("Accounts:", len(SharedData.accounts))

        SharedData.teams = []
        for company in SharedData.companies:
            response = requests.get(COMPANY_URL + f"/v1/companies/{company['uuid']}/teams", verify=False)
            company_teams = response.json()['teams']
            if len(company_teams) > 0:
                SharedData.teams.append((company['uuid'], company_teams))
        print("Teams:", len(SharedData.teams))

        num_workers = 0
        SharedData.workers = []
        for item in SharedData.teams:
            company_uuid, teams = item
            for team in teams:
                response = requests.get(COMPANY_URL +
                                        f"/v1/companies/{company_uuid}/teams/{team['uuid']}/workers",
                                        verify=False)
                if not 'error' in response.json():
                    this_workers = response.json()['workers']
                    if len(this_workers) > 0:
                        num_workers += len(this_workers)
                        SharedData.workers.append((company_uuid, team, this_workers))
                else:
                    print("Error:", response.json())
        print("Workers:", len(SharedData.workers))

        num_jobs = 0
        SharedData.jobs = []
        for item in SharedData.teams:
            company_uuid, teams = item
            for team in teams:
                response = requests.get(COMPANY_URL +
                                        f"/v1/companies/{company_uuid}/teams/{team['uuid']}/jobs",
                                        verify=False)
                if not 'error' in response.json():
                    this_jobs = response.json()['jobs']
                    if len(this_jobs) > 0:
                        num_jobs += len(this_jobs)
                        SharedData.jobs.append((company_uuid, team, this_jobs))
        print("Jobs:", len(SharedData.jobs))

        SharedData.initialized = False

    @tag('read')
    @task
    def get_workers_in_team(self):
        company_uuid, teams = random.choice(SharedData.teams)
        team = random.choice(teams)
        response = self.client.get(COMPANY_URL + f"/v1/companies/{company_uuid}/teams/{team['uuid']}/workers",
                                   verify=False,
                                   name="get_workers_in_team")

    @task
    @tag('read')
    def get_worker_team_info(self):
        _, _, workers = random.choice(SharedData.workers)
        user_uuid = random.choice(workers)['user_uuid']
        response = self.client.get(COMPANY_URL + f"/v1/companies/{user_uuid}/teaminfo",
                                   verify=False,
                                   name="get_worker_team_info")

    @task
    @tag('read')
    def list_teams(self):
        company_uuid = random.choice(SharedData.companies)['uuid']
        response = self.client.get(COMPANY_URL + f"/v1/companies/{company_uuid}/teams",
                                   verify=False,
                                   name="list_teams")

    @task
    @tag('update')
    @tag('task1')
    def update_team(self):
        company_uuid, team, _ = random.choice(SharedData.workers)
        if random.choice([0, 1]) == 0:
            team['color'] = (random.choice(['FF', 'DC', '04', '56']) +
                             random.choice(['FF', 'DC', '04', '56']) +
                             random.choice(['FF', 'DC', '04', '56']))
        else:
            team['day_week_starts'] = random.choice(['monday', 'tuesday', 'wednesday', 'thursday',
                                                     'friday', 'saturday', 'sunday'])
        response = self.client.put(COMPANY_URL + f"/v1/companies/{company_uuid}/teams/{team['uuid']}",
                                   json=team,
                                   verify=False,
                                   name="update_team")

    @task
    @tag('read')
    def get_job_list(self):
        company_uuid, teams = random.choice(SharedData.teams)
        team = random.choice(teams)
        response = self.client.get(COMPANY_URL + f"/v1/companies/{company_uuid}/teams/{team['uuid']}/jobs",
                                   verify=False,
                                   name="get_job_list")

    @task
    @tag('read')
    def get_directory_list(self):
        company_uuid = random.choice(SharedData.companies)['uuid']
        response = self.client.get(COMPANY_URL + f"/v1/companies/{company_uuid}/directory",
                                   verify=False,
                                   name="get_directory_list")

    @task
    @tag('read')
    def get_directory_entry(self):
        company_uuid, _, workers = random.choice(SharedData.workers)
        user_uuid = random.choice(workers)['user_uuid']
        response = self.client.get(COMPANY_URL + f"/v1/companies/{company_uuid}/directory/{user_uuid}",
                                   verify=False,
                                   name="get_directory_entry")

    @task
    @tag('read')
    def get_associations(self):
        company_uuid = random.choice(SharedData.companies)['uuid']
        response = self.client.get(COMPANY_URL + f"/v1/companies/{company_uuid}/associations",
                                   verify=False,
                                   name="get_associations")

    @task
    @tag('read')
    def list_companies(self):
        response = self.client.get(COMPANY_URL + f"/v1/companies",
                                   verify=False,
                                   name="list_companies")

    @task
    @tag('read')
    def list_admins(self):
        company_uuid = random.choice(SharedData.companies)['uuid']
        response = self.client.get(COMPANY_URL + f"/v1/companies/{company_uuid}/admins",
                                   verify=False,
                                   name="list_admins")

    # @task
    # @tag('read')
    # @tag('task1')
    # def get_worker_of(self):
    #     company_uuid, _, workers = random.choice(SharedData.workers)
    #     user_uuid = random.choice(workers)['user_uuid']
    #     response = self.client.get(COMPANY_URL + f"/v1/companies/{user_uuid}/info",
    #                                verify=False,
    #                                name="get_worker_of")

    # @task
    # @tag('read')
    # def get_admin_of(self):
    #     company_uuid, _, workers = random.choice(SharedData.workers)
    #     user_uuid = random.choice(workers)['user_uuid']
    #     response = self.client.get(COMPANY_URL + f"/v1/companies/{user_uuid}/admin_info",
    #                                verify=False,
    #                                name="get_admin_of")


    # @task
    # @tag('update')
    # def delete_admin(self):
    #     company_uuid, _, workers = random.choice(SharedData.workers)
    #     user_uuid = random.choice(workers)['user_uuid']
    #     response = self.client.delete(COMPANY_URL + f"/v1/companies/{company_uuid}/admins/{user_uuid}",
    #                                   verify=False,
    #                                   name="delete_admin")

    @task
    @tag('update')
    @tag('task1')
    @tag('task3')
    def delete_worker(self):
        company_uuid, team, workers = random.choice(SharedData.workers)
        user_uuid = random.choice(workers)['user_uuid']
        response = self.client.delete(COMPANY_URL + f"/v1/companies/{company_uuid}/teams/{team['uuid']}/workers/{user_uuid}",
                                      verify=False,
                                      name="delete_worker")

    @task
    @tag('read')
    @tag('task2')
    def list_jobs(self):
        company_uuid, teams = random.choice(SharedData.teams)
        team = random.choice(teams)
        response = self.client.get(COMPANY_URL + f"/v1/companies/{company_uuid}/teams/{team['uuid']}/jobs",
                                   verify=False,
                                   name="list_jobs")
        print(json.loads(response.content))

    @task
    @tag('update')
    @tag('task2')
    def update_job(self):
        company_uuid, team, jobs = random.choice(SharedData.jobs)
        job = random.choice(jobs)
        if random.choice([0, 1]) == 1:
            job['color'] = (random.choice(['FF', 'DC', '04', '56']) +
                            random.choice(['FF', 'DC', '04', '56']) +
                            random.choice(['FF', 'DC', '04', '56']))
        else:
            company_uuid, teams = random.choice(SharedData.teams)
            team = random.choice(teams)
            job['team_uuid'] = team['uuid']
        response = self.client.put(COMPANY_URL + f"/v1/companies/{company_uuid}/teams/{team['uuid']}/jobs/{job['uuid']}",
                                   json=job,
                                   verify=False,
                                   name="update_job")

    @task
    @tag('update')
    @tag('task3')
    def update_account(self):
        account = random.choice(SharedData.accounts)
        if random.choice([0, 1]) == 1:
            account['phonenumber'] = '+' + str(1000000000 + random.randint(0, 8999999999))
        else:
            account['email'] = 'account_em_' + str(random.randint(0, 8999999999)) + '@server.com'
        response = self.client.put(ACCOUNT_URL + f"/v1/accounts/{account['uuid']}",
                                   json=account,
                                   verify=False,
                                   name="update_account")

    @task
    @tag('read')
    @tag('task3')
    def list_workers(self):
        company_uuid, team, _ = random.choice(SharedData.workers)
        response = self.client.get(COMPANY_URL + f"/v1/companies/{company_uuid}/teams/{team['uuid']}/workers",
                                   verify=False,
                                   name="list_workers")
