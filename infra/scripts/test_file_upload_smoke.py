import unittest

from file_upload_smoke import make_tender_docx, make_tender_pdf, require_origin
import io
import zipfile


class PublicFileOriginTest(unittest.TestCase):
    def test_chinese_docx_fixture_carries_exact_facts_and_end_clause(self):
        with zipfile.ZipFile(io.BytesIO(make_tender_docx('unit-marker'))) as document:
            self.assertIsNone(document.testzip())
            xml = document.read('word/document.xml').decode()
        for value in ('2026-11-15 09:30', '市政公用工程施工总承包三级', '安全生产许可证', '技术方案40分', 'GRAY-END-CLAUSE-20261003', 'unit-marker'):
            self.assertIn(value, xml)
        self.assertNotIn('工程量清单', xml)
    def test_pdf_fixture_contains_valid_cross_reference_offset_and_project_marker(self):
        pdf = make_tender_pdf("ZBT-unit-test")
        self.assertTrue(pdf.startswith(b"%PDF-1.4"))
        self.assertIn(b"Tender project: ZBT-unit-test", pdf)
        xref = int(pdf.split(b"startxref\n")[1].splitlines()[0])
        self.assertEqual(pdf[xref:xref + 4], b"xref")

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
