import unittest
from full_bid_smoke import review_fixture_text


class FixtureReviewTests(unittest.TestCase):
    def test_fixture_review_removes_promises_without_inventing_replacements(self):
        text = '项目预算128万元。\n【事实待核实：质保承诺缺少依据。】'
        reviewed = review_fixture_text(text)
        self.assertIn('项目预算128万元。', reviewed)
        self.assertIn('相关承诺未作出', reviewed)
        self.assertNotIn('【事实待核实：', reviewed)
        self.assertNotIn('12个月', reviewed)

    def test_manual_fixture_review_does_not_touch_supported_text(self):
        text = '投标截止时间2026-11-15 09:30，合同生效后30天内交付。'
        self.assertEqual(review_fixture_text(text), text)
