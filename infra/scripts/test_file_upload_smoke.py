import unittest

from file_upload_smoke import require_origin


class PublicFileOriginTest(unittest.TestCase):
    def test_same_origin_with_port_and_signature_is_allowed(self):
        require_origin("http://ecs:8080/zbt-files/tenant/file?X-Amz-Signature=test", "http://ecs:8080")

    def test_inaccessible_storage_port_is_rejected(self):
        with self.assertRaises(RuntimeError):
            require_origin("http://ecs:19000/zbt-files/tenant/file", "http://ecs:8080")

    def test_other_scheme_host_or_bucket_is_rejected(self):
        for url in ("https://ecs:8080/zbt-files/file", "http://other:8080/zbt-files/file",
                    "http://ecs:8080/other-bucket/file", "http://ecs:8080/zbt-files-bad/file"):
            with self.subTest(url=url), self.assertRaises(RuntimeError):
                require_origin(url, "http://ecs:8080")


if __name__ == "__main__":
    unittest.main()
