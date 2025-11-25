import uuid
from locust import HttpUser, task, between


TEAM_NAME = "payments"


class ReviewerServiceUser(HttpUser):
    wait_time = between(0.2, 0.2)

    def on_start(self):
        payload = {
            "team_name": TEAM_NAME,
            "members": [
                {"user_id": "u1", "username": "Alice", "is_active": True},
                {"user_id": "u2", "username": "Bob", "is_active": True},
                {"user_id": "u3", "username": "Cara", "is_active": True},
                {"user_id": "u4", "username": "Dave", "is_active": True},
            ],
        }

        with self.client.post(
            "/team/add",
            json=payload,
            name="team_add",
            catch_response=True,
        ) as res:
            # считаем успехом и первый 200, и повторный 400 (TEAM_EXISTS)
            if res.status_code in (200, 400):
                res.success()
            else:
                res.failure(f"Unexpected status code for team_add: {res.status_code}, body={res.text}")

    @task
    def create_pr_and_reassign(self):
        """
        Нагружает два основных сценария:
        1) POST /pullRequest/create
        2) POST /pullRequest/reassign (если есть хотя бы один ревьюер)
        """
        pr_id = f"pr-{uuid.uuid4()}"
        author_id = "u1"

        create_payload = {
            "pull_request_id": pr_id,
            "pull_request_name": f"Test {pr_id}",
            "author_id": author_id,
        }

        with self.client.post(
            "/pullRequest/create",
            json=create_payload,
            name="create_pr",
            catch_response=True,
        ) as res:
            if res.status_code != 201:
                res.failure(f"Expected 201 on create_pr, got {res.status_code}, body={res.text}")
                return
            else:
                res.success()

            try:
                body = res.json()
            except Exception as e:
                res.failure(f"Failed to parse JSON on create_pr: {e}, body={res.text}")
                return

            pr = body.get("pr") or {}
            reviewers = pr.get("assigned_reviewers") or []

        if not reviewers:
            return

        old_user_id = reviewers[0]

        reassign_payload = {
            "pull_request_id": pr_id,
            "old_user_id": old_user_id,
        }

        with self.client.post(
            "/pullRequest/reassign",
            json=reassign_payload,
            name="reassign",
            catch_response=True,
        ) as res:
            # что считаем успехом:
            #  - 200: пере-назначили
            #  - 409: ожидаемый бизнес-конфликт (например, нет кандидатов)
            if res.status_code in (200, 409):
                res.success()
            else:
                res.failure(f"Unexpected status on reassign: {res.status_code}, body={res.text}")
