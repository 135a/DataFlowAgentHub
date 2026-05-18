import { useEffect } from "react";
import { useNavigate } from "react-router-dom";

export function TablesPage() {
  const navigate = useNavigate();

  useEffect(() => {
    navigate("/datasets", { replace: true });
  }, [navigate]);

  return null;
}
