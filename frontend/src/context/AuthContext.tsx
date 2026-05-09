import { createContext, useContext, useEffect, useState, type ReactNode } from "react";
import pb from "../lib/pb";
import type { RecordModel } from "pocketbase";

export type UserRole = "staff" | "customer";

interface AuthState {
  user: RecordModel | null;
  role: UserRole | null;
  isLoading: boolean;
}

interface AuthContextValue extends AuthState {
  requestStaffOTP: (email: string) => Promise<string>;
  requestCustomerOTP: (email: string) => Promise<string>;
  loginAsStaff: (otpId: string, code: string) => Promise<void>;
  loginAsCustomer: (otpId: string, code: string) => Promise<void>;
  logout: () => void;
}

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<AuthState>({
    user: null,
    role: null,
    isLoading: true,
  });

  useEffect(() => {
    // Restore session from localStorage on mount
    if (pb.authStore.isValid && pb.authStore.record) {
      const collection = pb.authStore.record.collectionName as string;
      setState({
        user: pb.authStore.record,
        role: collection === "users" ? "staff" : "customer",
        isLoading: false,
      });
    } else {
      setState((s) => ({ ...s, isLoading: false }));
    }

    // Keep state in sync if the store changes externally (e.g. token expiry)
    const unsub = pb.authStore.onChange(() => {
      if (!pb.authStore.isValid) {
        setState({ user: null, role: null, isLoading: false });
      }
    });
    return unsub;
  }, []);

  async function requestStaffOTP(email: string): Promise<string> {
    const result = await pb.collection("users").requestOTP(email);
    return result.otpId;
  }

  async function requestCustomerOTP(email: string): Promise<string> {
    const result = await pb.collection("customers").requestOTP(email);
    return result.otpId;
  }

  async function loginAsStaff(otpId: string, code: string) {
    await pb.collection("users").authWithOTP(otpId, code);
    setState({ user: pb.authStore.record, role: "staff", isLoading: false });
  }

  async function loginAsCustomer(otpId: string, code: string) {
    await pb.collection("customers").authWithOTP(otpId, code);
    setState({ user: pb.authStore.record, role: "customer", isLoading: false });
  }

  function logout() {
    pb.authStore.clear();
    setState({ user: null, role: null, isLoading: false });
  }

  return (
    <AuthContext.Provider
      value={{
        ...state,
        requestStaffOTP,
        requestCustomerOTP,
        loginAsStaff,
        loginAsCustomer,
        logout,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used inside AuthProvider");
  return ctx;
}
